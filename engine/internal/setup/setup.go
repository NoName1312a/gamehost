// Package setup powers the first-run guided wizard. It detects the host
// prerequisites for running game containers (WSL2 + Docker on Windows) and can
// launch the fix for each missing step. Detection is cross-platform-aware; the
// install/enable fixes are Windows-only and trigger a UAC elevation prompt,
// which is how a non-elevated process (the engine, launched by the desktop
// shell) asks the user for admin rights.
package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/leop1/gamehost/engine/internal/docker"
)

// dockerDesktopPath is the default Docker Desktop install location on Windows.
const dockerDesktopPath = `C:\Program Files\Docker\Docker\Docker Desktop.exe`

// Step IDs (status/display) and action IDs (endpoint suffixes).
const (
	stepWSL2            = "wsl2"
	stepDockerInstalled = "docker-installed"
	stepDockerRunning   = "docker-running"

	ActionEnableWSL     = "enable-wsl"
	ActionInstallDocker = "install-docker"
	ActionStartDocker   = "start-docker"
)

// ErrUnsupported is returned when an automated fix is requested on a platform
// that doesn't support it (everything but Windows, for now).
var ErrUnsupported = errors.New("automated setup is only available on the Windows desktop app")

// Action describes the one-click fix for a not-yet-satisfied step.
type Action struct {
	Label       string `json:"label"`
	Endpoint    string `json:"endpoint"` // POST here to run it
	Command     string `json:"command"`  // copy-paste fallback shown in the UI
	NeedsAdmin  bool   `json:"needsAdmin"`
	NeedsReboot bool   `json:"needsReboot"`
}

// Step is one prerequisite in the wizard.
type Step struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Status string  `json:"status"` // "ok" | "todo"
	Detail string  `json:"detail"`
	Action *Action `json:"action,omitempty"`
}

// Report is the full prerequisite state surfaced to the UI.
type Report struct {
	Platform string `json:"platform"`
	Ready    bool   `json:"ready"` // true once a container runtime is reachable
	Steps    []Step `json:"steps"`
}

// Result reports the outcome of launching a fix action.
type Result struct {
	Started     bool   `json:"started"`
	NeedsReboot bool   `json:"needsReboot,omitempty"`
	Hint        string `json:"hint"`
}

// Detect builds the prerequisite report for the current host. It never errors:
// "not installed" / "not running" are normal states the wizard surfaces.
func Detect(ctx context.Context, rt *docker.Runtime) Report {
	running := rt.Probe(ctx).Connected
	if runtime.GOOS == "windows" {
		return detectWindows(ctx, running)
	}
	return detectGeneric(running)
}

func detectWindows(ctx context.Context, dockerRunning bool) Report {
	wsl := wslAvailable(ctx)
	installed := dockerInstalled()

	steps := []Step{
		{
			ID:     stepWSL2,
			Title:  "Enable WSL2",
			Status: okTodo(wsl),
			Detail: pick(wsl,
				"Windows Subsystem for Linux is ready.",
				"Docker Desktop runs on WSL2. We'll enable it for you — this usually needs a restart."),
			Action: actionIf(!wsl, &Action{
				Label:       "Enable WSL2",
				Endpoint:    "/api/system/setup/" + ActionEnableWSL,
				Command:     "wsl --install",
				NeedsAdmin:  true,
				NeedsReboot: true,
			}),
		},
		{
			ID:     stepDockerInstalled,
			Title:  "Install Docker Desktop",
			Status: okTodo(installed),
			Detail: pick(installed,
				"Docker Desktop is installed.",
				"Game servers run in Docker containers. We'll install Docker Desktop for you."),
			Action: actionIf(!installed, &Action{
				Label:      "Install Docker Desktop",
				Endpoint:   "/api/system/setup/" + ActionInstallDocker,
				Command:    "winget install -e --id Docker.DockerDesktop",
				NeedsAdmin: true,
			}),
		},
		{
			ID:     stepDockerRunning,
			Title:  "Start Docker",
			Status: okTodo(dockerRunning),
			Detail: pick(dockerRunning,
				"Docker engine is running. You're ready to host.",
				"Docker is installed but not running yet. We'll start it — first launch can take a minute."),
			// Only offer "Start" once Docker is actually installed.
			Action: actionIf(!dockerRunning && installed, &Action{
				Label:    "Start Docker",
				Endpoint: "/api/system/setup/" + ActionStartDocker,
				Command:  `Launch "Docker Desktop" from the Start menu`,
			}),
		},
	}
	return Report{Platform: "windows", Ready: dockerRunning, Steps: steps}
}

func detectGeneric(dockerRunning bool) Report {
	installed := lookDocker()
	steps := []Step{
		{
			ID:     stepDockerInstalled,
			Title:  "Install Docker",
			Status: okTodo(installed),
			Detail: pick(installed, "Docker is installed.", "Install Docker (or a compatible container runtime) to host servers."),
		},
		{
			ID:     stepDockerRunning,
			Title:  "Start Docker",
			Status: okTodo(dockerRunning),
			Detail: pick(dockerRunning, "Docker engine is running.", "Start the Docker daemon to host servers."),
		},
	}
	return Report{Platform: runtime.GOOS, Ready: dockerRunning, Steps: steps}
}

// RunAction launches the fix for the given action ID. The install/enable
// actions are Windows-only; start-docker just launches the app.
func RunAction(id string) (Result, error) {
	switch id {
	case ActionEnableWSL:
		if runtime.GOOS != "windows" {
			return Result{}, ErrUnsupported
		}
		if err := launchElevated("wsl", "--install"); err != nil {
			return Result{}, err
		}
		return Result{Started: true, NeedsReboot: true,
			Hint: "Approve the Windows prompt. If asked, restart your PC and reopen GameNest."}, nil
	case ActionInstallDocker:
		if runtime.GOOS != "windows" {
			return Result{}, ErrUnsupported
		}
		if err := launchElevated("winget", "install", "-e", "--id", "Docker.DockerDesktop"); err != nil {
			return Result{}, err
		}
		return Result{Started: true,
			Hint: "Approve the Windows prompt and let the installer finish, then click Recheck."}, nil
	case ActionStartDocker:
		if runtime.GOOS != "windows" {
			return Result{}, ErrUnsupported
		}
		if err := launchDockerDesktop(); err != nil {
			return Result{}, err
		}
		return Result{Started: true,
			Hint: "Starting Docker Desktop — first launch can take a minute."}, nil
	default:
		return Result{}, fmt.Errorf("unknown setup step %q", id)
	}
}

// --- detection helpers ------------------------------------------------------

func lookDocker() bool { _, err := exec.LookPath("docker"); return err == nil }

func dockerInstalled() bool {
	if lookDocker() {
		return true
	}
	_, err := os.Stat(dockerDesktopPath)
	return err == nil
}

// wslAvailable reports whether WSL is installed and configured. `wsl --status`
// exits 0 when the feature is present; it errors when WSL isn't enabled.
func wslAvailable(ctx context.Context) bool {
	if _, err := exec.LookPath("wsl"); err != nil {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return exec.CommandContext(cctx, "wsl", "--status").Run() == nil
}

// --- launch helpers ---------------------------------------------------------

// launchElevated fires off `Start-Process -Verb RunAs` (which raises a UAC
// prompt) and returns immediately — the elevated installer runs independently.
func launchElevated(file string, args ...string) error {
	return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", buildStartProcess(file, args)).Start()
}

func launchDockerDesktop() error {
	if _, err := os.Stat(dockerDesktopPath); err != nil {
		return fmt.Errorf("Docker Desktop not found at %s", dockerDesktopPath)
	}
	return exec.Command(dockerDesktopPath).Start()
}

// buildStartProcess assembles a PowerShell Start-Process command that runs the
// target elevated, e.g. `Start-Process -FilePath 'wsl' -ArgumentList '--install' -Verb RunAs`.
func buildStartProcess(file string, args []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Start-Process -FilePath '%s'", psEscape(file))
	if len(args) > 0 {
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = "'" + psEscape(a) + "'"
		}
		fmt.Fprintf(&b, " -ArgumentList %s", strings.Join(quoted, ","))
	}
	b.WriteString(" -Verb RunAs")
	return b.String()
}

func psEscape(s string) string { return strings.ReplaceAll(s, "'", "''") }

// --- tiny helpers -----------------------------------------------------------

func okTodo(ok bool) string {
	if ok {
		return "ok"
	}
	return "todo"
}

func pick(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

func actionIf(cond bool, a *Action) *Action {
	if cond {
		return a
	}
	return nil
}
