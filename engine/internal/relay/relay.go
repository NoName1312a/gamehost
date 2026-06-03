// Package relay integrates playit.gg as a no-port-forwarding connectivity
// option, for routers that block UPnP or networks behind CGNAT. The playit
// agent (a daemon) maintains outbound tunnels that friends connect to; this
// package is the thin glue around it: locate/install the binary, report whether
// it's linked to an account and running, supervise the daemon, and open the
// setup/dashboard pages. Tunnels themselves are created by the user on
// playit.gg (their CLI is Linux-only and the API is undocumented), so each
// server stores the address the user copies back from the dashboard.
package relay

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	SetupURL     = "https://playit.gg/"
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

// Agent locates and supervises the playit daemon.
type Agent struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
}

func New() *Agent { return &Agent{} }

// Status reports installed/linked/running plus the setup + dashboard URLs.
func (a *Agent) Status() Status {
	st := Status{
		Installed:    locate() != "",
		Linked:       linked(),
		Running:      a.Running(),
		SetupURL:     SetupURL,
		DashboardURL: DashboardURL,
	}
	switch {
	case !st.Installed:
		st.Message = "Install the playit relay to let friends connect without port-forwarding."
	case !st.Linked:
		st.Message = "Link your free playit account once, then create a tunnel to your server's port."
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

// Start runs the playit daemon (it loads its secret from the default path). A
// no-op if already running; errors if not installed/linked.
func (a *Agent) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		return nil
	}
	bin := locate()
	if bin == "" {
		return errNotInstalled
	}
	if !linked() {
		return errNotLinked
	}
	cmd := exec.Command(bin)
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

// linked reports whether the agent has been claimed (secret file present).
func linked() bool { return exists(secretPath()) }

func secretPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "playit_gg", "playit.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "playit_gg", "playit.toml")
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// RunAction dispatches a UI-triggered relay action.
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
