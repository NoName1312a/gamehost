package docker

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ImageExists reports whether an image is already present locally (so the
// caller can decide whether a pull — and its progress UI — is needed).
func (r *Runtime) ImageExists(ctx context.Context, image string) bool {
	_, err := r.run(ctx, "image", "inspect", image)
	return err == nil
}

// Pull pulls an image, reporting progress via onProgress (a 0-100 percent and a
// short status). Progress is layer-granularity: the docker CLI doesn't emit
// byte-level progress without a TTY, so the bar advances as each layer arrives.
func (r *Runtime) Pull(ctx context.Context, image string, onProgress func(percent int, status string)) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", image)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var errb strings.Builder
	cmd.Stderr = &errb
	if err := cmd.Start(); err != nil {
		return err
	}

	total, done := 0, 0
	report := func() {
		pct := 0
		if total > 0 {
			pct = done * 100 / total
			if pct > 100 {
				pct = 100
			}
		}
		if onProgress != nil {
			onProgress(pct, fmt.Sprintf("Downloading game files… (%d/%d layers)", done, total))
		}
	}
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.Contains(line, "Already exists"):
			total++
			done++
		case strings.Contains(line, "Pulling fs layer"):
			total++
		case strings.Contains(line, "Pull complete"):
			done++
		default:
			continue
		}
		report()
	}
	if err := cmd.Wait(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("docker pull %s: %s", image, msg)
	}
	return nil
}
