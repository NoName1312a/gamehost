package tunnel

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var errNoFrpc = errors.New("frpc binary not found")

// execCommand is exec.Command, indirected so tests can substitute a fake frpc.
var execCommand = exec.Command

// localProxy is one frpc proxy: a local host port exposed at a remote public
// port, bound to its allocation's secret via metadatas.gnsecret.
type localProxy struct {
	Name       string
	Proto      string // "tcp" | "udp"
	LocalPort  int    // host port the game server listens on (127.0.0.1)
	RemotePort int    // public port on frps
	Secret     string // per-allocation secret (proves device ownership)
}

// locate finds the frpc binary: an explicit override, then PATH, then alongside
// the engine binary (where the Tauri bundle drops it).
func locate() string {
	if p := os.Getenv("GAMEHOST_FRPC"); p != "" && exists(p) {
		return p
	}
	if p, err := exec.LookPath("frpc"); err == nil {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		if cand := filepath.Join(filepath.Dir(exe), frpcName()); exists(cand) {
			return cand
		}
	}
	return ""
}

func frpcName() string {
	if runtime.GOOS == "windows" {
		return "frpc.exe"
	}
	return "frpc"
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// splitFrps splits a "host:port" frps address; on any parse failure it falls
// back to the whole string as host and frp's default bind port.
func splitFrps(addr string) (host string, port int) {
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 7000
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return h, 7000
	}
	return h, n
}

// writeConfig renders a frpc.toml for frp v0.69.x with one [[proxies]] block per
// proxy. All proxies share the frps coordinates; each carries its own secret.
func writeConfig(path, frpsAddr, frpsToken string, proxies []localProxy) error {
	host, port := splitFrps(frpsAddr)
	var b strings.Builder
	fmt.Fprintf(&b, "serverAddr = %q\n", host)
	fmt.Fprintf(&b, "serverPort = %d\n", port)
	b.WriteString("auth.method = \"token\"\n")
	fmt.Fprintf(&b, "auth.token = %q\n", frpsToken)
	for _, p := range proxies {
		b.WriteString("\n[[proxies]]\n")
		fmt.Fprintf(&b, "name = %q\n", p.Name)
		fmt.Fprintf(&b, "type = %q\n", p.Proto)
		b.WriteString("localIP = \"127.0.0.1\"\n")
		fmt.Fprintf(&b, "localPort = %d\n", p.LocalPort)
		fmt.Fprintf(&b, "remotePort = %d\n", p.RemotePort)
		fmt.Fprintf(&b, "metadatas.gnsecret = %q\n", p.Secret)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

// sidecar supervises a single frpc child process (restart-on-change for MVP).
type sidecar struct {
	bin string

	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
}

// restart stops any running frpc and starts a fresh one against cfgPath.
func (s *sidecar) restart(cfgPath string) error {
	s.stop()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bin == "" {
		return errNoFrpc
	}
	cmd := execCommand(s.bin, "-c", cfgPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	s.cmd = cmd
	s.running = true
	go func(c *exec.Cmd) {
		_ = c.Wait()
		s.mu.Lock()
		// Only clear running if this is still the current process, so a stale
		// Wait from a replaced process can't mark a live frpc as stopped.
		if s.cmd == c {
			s.running = false
		}
		s.mu.Unlock()
	}(cmd)
	return nil
}

// stop kills the supervised frpc, if any.
func (s *sidecar) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.cmd = nil
	s.running = false
}

func (s *sidecar) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// pid returns the current frpc process id, or 0 if not running.
func (s *sidecar) pid() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Pid
	}
	return 0
}
