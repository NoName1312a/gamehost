// Package docker wraps the container runtime. For M0 it only probes whether a
// Docker engine is reachable, via the docker CLI — this keeps the scaffold free
// of the (currently churning) Docker Go SDK. M1 swaps this for the SDK to do
// real container lifecycle, console streaming, and resource limits.
package docker

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Status reports whether the engine can talk to a container runtime. The UI
// uses Connected to decide whether to show the setup wizard.
type Status struct {
	Connected     bool   `json:"connected"`
	ServerVersion string `json:"serverVersion,omitempty"`
	Message       string `json:"message"`
}

// Runtime is a thin handle over the container runtime.
type Runtime struct{}

// New constructs a Runtime.
func New() *Runtime { return &Runtime{} }

// Probe reports the Docker engine status. It never errors: "not installed" and
// "installed but not running" are normal states the UI surfaces as setup steps.
func (r *Runtime) Probe(ctx context.Context) Status {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	if _, err := exec.LookPath("docker"); err != nil {
		return Status{Message: "Docker isn't installed yet. Install Docker Desktop to start hosting servers."}
	}

	// Querying the *server* version requires a running daemon.
	out, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return Status{Message: "Docker is installed, but the engine isn't running. Start Docker Desktop and retry."}
	}

	v := strings.TrimSpace(string(out))
	if v == "" {
		return Status{Message: "Docker engine not reachable. Start Docker Desktop and retry."}
	}
	return Status{Connected: true, ServerVersion: v, Message: "Docker engine connected."}
}
