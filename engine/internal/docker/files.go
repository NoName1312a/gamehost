package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

// fileHelperImage is a tiny image used to read/write a server's data volume
// without depending on the game container running. Multiple containers can
// mount the same named volume, so this works alongside a live server.
const fileHelperImage = "busybox:latest"

// MaxEditableBytes caps in-app text editing so we never pull a huge/binary file
// into memory or the editor.
const MaxEditableBytes = 1 << 20 // 1 MiB

// FileEntry is one item in a server's data directory.
type FileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// containerPath cleans a user-supplied relative path to an absolute path under
// /data, preventing traversal outside the volume root (path.Clean collapses
// any "..", and the result can never rise above "/").
func containerPath(rel string) string {
	clean := path.Clean("/" + strings.ReplaceAll(rel, "\\", "/"))
	if clean == "/" {
		return "/data"
	}
	return "/data" + clean
}

// volRun runs a one-shot busybox command with the volume mounted at /data. The
// target path is passed as $1 (positional arg, never interpolated into the
// script) so filenames can't inject shell. stdin, if non-nil, is piped in.
func (r *Runtime) volRun(ctx context.Context, volume string, stdin []byte, script, arg string) (string, error) {
	args := []string{"run", "--rm"}
	if stdin != nil {
		args = append(args, "-i")
	}
	args = append(args, "-v", volume+":/data", fileHelperImage, "sh", "-c", script, "_", arg)
	cmd := exec.CommandContext(ctx, "docker", args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("file operation failed: %s", msg)
	}
	return out.String(), nil
}

// ListFiles returns the entries directly under rel within the volume.
func (r *Runtime) ListFiles(ctx context.Context, volume, rel string) ([]FileEntry, error) {
	script := `cd "$1" 2>/dev/null || exit 0; for e in * .[!.]*; do [ -e "$e" ] || continue; if [ -d "$e" ]; then printf 'd\t0\t%s\n' "$e"; else printf 'f\t%s\t%s\n' "$(stat -c %s "$e" 2>/dev/null || echo 0)" "$e"; fi; done`
	out, err := r.volRun(ctx, volume, nil, script, containerPath(rel))
	if err != nil {
		return nil, err
	}
	entries := []FileEntry{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		size, _ := strconv.ParseInt(parts[1], 10, 64)
		entries = append(entries, FileEntry{Name: parts[2], IsDir: parts[0] == "d", Size: size})
	}
	return entries, nil
}

// ReadFile returns a file's contents, truncated to MaxEditableBytes (the second
// return value reports whether truncation happened).
func (r *Runtime) ReadFile(ctx context.Context, volume, rel string) (string, bool, error) {
	cp := containerPath(rel)
	kind, err := r.volRun(ctx, volume, nil, `if [ -d "$1" ]; then echo DIR; elif [ -f "$1" ]; then stat -c %s "$1"; else echo NONE; fi`, cp)
	if err != nil {
		return "", false, err
	}
	kind = strings.TrimSpace(kind)
	switch kind {
	case "DIR":
		return "", false, errors.New("that path is a folder, not a file")
	case "NONE":
		return "", false, errors.New("file not found")
	}
	size, _ := strconv.ParseInt(kind, 10, 64)
	script, truncated := `cat "$1"`, false
	if size > MaxEditableBytes {
		script, truncated = fmt.Sprintf(`head -c %d "$1"`, MaxEditableBytes), true
	}
	content, err := r.volRun(ctx, volume, nil, script, cp)
	if err != nil {
		return "", false, err
	}
	return content, truncated, nil
}

// WriteFile writes content to a file, creating parent directories as needed.
func (r *Runtime) WriteFile(ctx context.Context, volume, rel string, content []byte) error {
	_, err := r.volRun(ctx, volume, content, `mkdir -p "$(dirname "$1")"; cat > "$1"`, containerPath(rel))
	return err
}

// MakeDir creates a directory (and any missing parents).
func (r *Runtime) MakeDir(ctx context.Context, volume, rel string) error {
	_, err := r.volRun(ctx, volume, nil, `mkdir -p "$1"`, containerPath(rel))
	return err
}

// DeleteFile removes a file or directory tree. Refuses to wipe the volume root.
func (r *Runtime) DeleteFile(ctx context.Context, volume, rel string) error {
	cp := containerPath(rel)
	if cp == "/data" {
		return errors.New("refusing to delete the data root")
	}
	_, err := r.volRun(ctx, volume, nil, `rm -rf "$1"`, cp)
	return err
}
