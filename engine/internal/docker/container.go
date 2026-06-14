package docker

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	// defaultPidsLimit caps processes/threads per container as a fork-bomb guard.
	// Generous enough for heavy Java servers (many JVM threads) while still well
	// below the host's pid_max, so a runaway container can't exhaust host PIDs.
	defaultPidsLimit = 4096
	// defaultNofile bounds per-container file descriptors.
	defaultNofile = 1048576
)

// PortMapping maps a host port to a container port.
type PortMapping struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol"`
}

// CreateSpec is everything needed to launch a game-server container.
type CreateSpec struct {
	Name     string
	Image    string
	Env      map[string]string
	Ports    []PortMapping
	MemoryMB int
	// CPUs caps CPU cores (e.g. 1.5). 0 leaves CPU uncapped.
	CPUs float64
	// PidsLimit caps the number of processes/threads (fork-bomb guard). 0 applies
	// the default (defaultPidsLimit); a negative value disables the cap.
	PidsLimit int
	Volume    string // named volume for persistent data
	DataPath  string // mount point inside the container
	OpenStdin bool
}

// State is the live status of a container.
type State struct {
	Exists  bool   `json:"exists"`
	Status  string `json:"status"`
	Running bool   `json:"running"`
}

func (r *Runtime) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("docker %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// RunArgs builds the `docker run` argument list for a spec. Exported and pure
// so it can be unit-tested without a running Docker engine. Env keys are sorted
// for deterministic output.
func RunArgs(spec CreateSpec) []string {
	args := []string{"run", "-d", "--name", spec.Name, "--restart", "unless-stopped"}
	if spec.OpenStdin {
		args = append(args, "-i")
	}
	if spec.MemoryMB > 0 {
		args = append(args, "-m", fmt.Sprintf("%dm", spec.MemoryMB))
	}
	if spec.CPUs > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(spec.CPUs, 'f', -1, 64))
	}
	// Fork-bomb guard: 0 applies the default cap; a negative value opts out.
	if pids := spec.PidsLimit; pids >= 0 {
		if pids == 0 {
			pids = defaultPidsLimit
		}
		args = append(args, "--pids-limit", strconv.Itoa(pids))
	}
	// Bound per-container file descriptors so one server can't exhaust the host's.
	args = append(args, "--ulimit", fmt.Sprintf("nofile=%d:%d", defaultNofile, defaultNofile))
	for _, k := range sortedKeys(spec.Env) {
		args = append(args, "-e", k+"="+spec.Env[k])
	}
	for _, p := range spec.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		args = append(args, "-p", fmt.Sprintf("%d:%d/%s", p.Host, p.Container, proto))
	}
	if spec.Volume != "" && spec.DataPath != "" {
		args = append(args, "-v", spec.Volume+":"+spec.DataPath)
	}
	args = append(args, spec.Image)
	return args
}

// Run creates and starts a container from a spec (pulls the image if missing).
func (r *Runtime) Run(ctx context.Context, spec CreateSpec) error {
	_, err := r.run(ctx, RunArgs(spec)...)
	return err
}

func (r *Runtime) Start(ctx context.Context, name string) error {
	_, err := r.run(ctx, "start", name)
	return err
}

func (r *Runtime) Stop(ctx context.Context, name string) error {
	_, err := r.run(ctx, "stop", name)
	return err
}

func (r *Runtime) Remove(ctx context.Context, name string) error {
	_, err := r.run(ctx, "rm", "-f", name)
	return err
}

func (r *Runtime) RemoveVolume(ctx context.Context, name string) error {
	_, err := r.run(ctx, "volume", "rm", name)
	return err
}

// Exec runs a one-shot command inside a running container and returns stdout.
func (r *Runtime) Exec(ctx context.Context, name string, cmd ...string) (string, error) {
	return r.run(ctx, append([]string{"exec", name}, cmd...)...)
}

// Inspect reports a container's live state. A missing container is reported as
// Exists:false (not an error).
func (r *Runtime) Inspect(ctx context.Context, name string) State {
	out, err := r.run(ctx, "inspect", "-f", "{{.State.Status}}", name)
	if err != nil {
		return State{Exists: false}
	}
	return State{Exists: true, Status: out, Running: out == "running"}
}

// LogsReader starts `docker logs -f` and returns a reader over its combined
// stdout/stderr. Either cancel ctx or Close the reader to stop streaming and
// kill the process — both are safe and idempotent.
func (r *Runtime) LogsReader(ctx context.Context, name string, tail int) (io.ReadCloser, error) {
	// Derive our own cancel so Close() can kill the process directly, instead of
	// relying on the caller threading this exact ctx into the kill path. Without
	// it, closing the reader alone would stop draining stdout, `docker logs -f`
	// would block on a full pipe and never exit, and cmd.Wait would leak.
	cctx, cancel := context.WithCancel(ctx)
	pr, pw := io.Pipe()
	cmd := exec.CommandContext(cctx, "docker", "logs", "-f", "--tail", fmt.Sprintf("%d", tail), name)
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		cancel()
		_ = pw.Close()
		return nil, err
	}
	go func() {
		_ = cmd.Wait()
		_ = pw.Close() // unblock the reader once the process is gone
	}()
	return &logsReadCloser{pr: pr, cancel: cancel}, nil
}

// logsReadCloser wraps the logs pipe so Close() both kills the `docker logs`
// process (via cancel) and unblocks the reader (via the pipe). Close is
// idempotent.
type logsReadCloser struct {
	pr     *io.PipeReader
	cancel context.CancelFunc
	once   sync.Once
}

func (l *logsReadCloser) Read(p []byte) (int, error) { return l.pr.Read(p) }

func (l *logsReadCloser) Close() error {
	l.once.Do(l.cancel)
	return l.pr.Close()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
