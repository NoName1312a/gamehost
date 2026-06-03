// Package relay integrates playit.gg as a no-port-forwarding connectivity
// option, for routers that block UPnP or networks behind CGNAT. The playit
// agent (a daemon) maintains outbound tunnels that friends connect to; this
// package is the thin glue around it: locate/install the binary, link it with a
// secret key, supervise the daemon, and open the setup/dashboard pages.
//
// On Windows the distributed binary is daemon-only (no interactive claim), so
// linking uses the headless secret-key method (same as playit's Docker setup):
// the user generates a secret key on playit.gg and pastes it in; we store it and
// run the daemon with `--secret`. Tunnels themselves are created by the user on
// the playit dashboard, so each server stores the address to share.
package relay

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	// SetupURL generates a secret key (the headless/Docker agent flow).
	SetupURL     = "https://playit.gg/account/agents/new-docker"
	DashboardURL = "https://playit.gg/account/tunnels"
	wingetID     = "DevelopedMethods.playit"
)

var (
	errNotInstalled       = errors.New("playit is not installed")
	errNotLinked          = errors.New("playit is not linked to an account yet")
	errUnsupportedInstall = errors.New("automatic install is only available on Windows")
)

// Status is the relay state surfaced to the UI.
type Status struct {
	Installed    bool   `json:"installed"`
	Linked       bool   `json:"linked"`
	Running      bool   `json:"running"`
	SetupURL     string `json:"setupUrl"`
	DashboardURL string `json:"dashboardUrl"`
	Message      string `json:"message"`
}

// Agent locates, links, and supervises the playit daemon.
type Agent struct {
	dataDir string

	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
}

// New returns an Agent that stores its secret under dataDir.
func New(dataDir string) *Agent { return &Agent{dataDir: dataDir} }

func (a *Agent) secretFile() string { return filepath.Join(a.dataDir, "playit-secret.txt") }

func (a *Agent) secret() string {
	b, err := os.ReadFile(a.secretFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (a *Agent) linked() bool { return a.secret() != "" }

// Status reports installed/linked/running plus the setup + dashboard URLs.
func (a *Agent) Status() Status {
	st := Status{
		Installed:    locate() != "",
		Linked:       a.linked(),
		Running:      a.Running(),
		SetupURL:     SetupURL,
		DashboardURL: DashboardURL,
	}
	switch {
	case !st.Installed:
		st.Message = "Install the playit relay to let friends connect without port-forwarding."
	case !st.Linked:
		st.Message = "Get a secret key from playit.gg and paste it here to link your account."
	default:
		st.Message = "playit is linked. Create a tunnel on the dashboard, then paste its address onto your server."
	}
	return st
}

func (a *Agent) Running() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// Link stores the secret key and (re)starts the daemon with it.
func (a *Agent) Link(secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return errors.New("secret key is empty")
	}
	if err := os.MkdirAll(a.dataDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(a.secretFile(), []byte(secret), 0o600); err != nil {
		return err
	}
	a.Stop()
	return a.Start()
}

// Start runs the playit daemon with the stored secret. No-op if already
// running; errors if not installed/linked.
func (a *Agent) Start() error {
	key := a.secret()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		return nil
	}
	bin := locate()
	if bin == "" {
		return errNotInstalled
	}
	if key == "" {
		return errNotLinked
	}
	cmd := exec.Command(bin, "--secret", key)
	if err := cmd.Start(); err != nil {
		return err
	}
	a.cmd = cmd
	a.running = true
	go func() {
		_ = cmd.Wait()
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}()
	return nil
}

// Stop kills the supervised daemon.
func (a *Agent) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cmd != nil && a.cmd.Process != nil {
		_ = a.cmd.Process.Kill()
	}
	a.running = false
}

// Install installs the playit agent via winget (user scope, no elevation).
// Fire-and-forget: returns once the installer is launched; the UI re-polls
// Status until Installed flips true.
func (a *Agent) Install() error {
	if runtime.GOOS != "windows" {
		return errUnsupportedInstall
	}
	return exec.Command("winget", "install", "-e", "--id", wingetID,
		"--accept-package-agreements", "--accept-source-agreements").Start()
}

// RunAction dispatches a UI-triggered relay action (no payload).
func (a *Agent) RunAction(ctx context.Context, action string) error {
	switch action {
	case "install":
		return a.Install()
	case "start":
		return a.Start()
	case "stop":
		a.Stop()
		return nil
	case "open-setup":
		return OpenURL(ctx, SetupURL)
	case "open-dashboard":
		return OpenURL(ctx, DashboardURL)
	default:
		return errors.New("unknown relay action")
	}
}

// OpenURL opens a URL in the user's default browser.
func OpenURL(ctx context.Context, u string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", u).Start()
	case "darwin":
		return exec.CommandContext(ctx, "open", u).Start()
	default:
		return exec.CommandContext(ctx, "xdg-open", u).Start()
	}
}

// locate finds the playit binary: an explicit override, then PATH, then the
// winget install locations.
func locate() string {
	if p := os.Getenv("GAMEHOST_PLAYIT"); p != "" && exists(p) {
		return p
	}
	if p, err := exec.LookPath("playit"); err == nil {
		return p
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		if link := filepath.Join(la, "Microsoft", "WinGet", "Links", "playit.exe"); exists(link) {
			return link
		}
		if m, _ := filepath.Glob(filepath.Join(la, "Microsoft", "WinGet", "Packages", wingetID+"*", "playit.exe")); len(m) > 0 {
			return m[0]
		}
	}
	return ""
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
