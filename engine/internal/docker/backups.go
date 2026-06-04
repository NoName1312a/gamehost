package docker

import (
	"context"
	"strconv"
	"strings"
)

// backupsVolume is a single shared Docker volume holding every server's
// backups at /backups/<serverID>/<file>.tar.gz. Keeping backups in a volume
// (rather than a bind-mounted host path) avoids Docker Desktop file-sharing
// pitfalls and works identically on a Linux self-host later.
const backupsVolume = "gamehost-backups"

// BackupInfo describes one stored backup archive.
type BackupInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// CreateBackup tars a server's data volume into backups:/backups/<id>/<file>.
func (r *Runtime) CreateBackup(ctx context.Context, serverVol, id, file string) error {
	dst := "/backups/" + id + "/" + file
	args := []string{
		"run", "--rm",
		"-v", serverVol + ":/data:ro",
		"-v", backupsVolume + ":/backups",
		fileHelperImage, "sh", "-c",
		`mkdir -p "$(dirname "$1")" && tar czf "$1" -C /data .`, "_", dst,
	}
	_, err := r.run(ctx, args...)
	return err
}

// ListBackups lists a server's backup archives (newest filenames sort last, as
// they're timestamped).
func (r *Runtime) ListBackups(ctx context.Context, id string) ([]BackupInfo, error) {
	args := []string{
		"run", "--rm", "-v", backupsVolume + ":/backups", fileHelperImage, "sh", "-c",
		`cd "/backups/$1" 2>/dev/null || exit 0; for f in *.tar.gz; do [ -e "$f" ] || continue; printf '%s\t%s\n' "$(stat -c %s "$f")" "$f"; done`,
		"_", id,
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	list := []BackupInfo{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		size, _ := strconv.ParseInt(parts[0], 10, 64)
		list = append(list, BackupInfo{Name: parts[1], Size: size})
	}
	return list, nil
}

// RestoreVolume wipes a server's data volume and extracts a backup into it. The
// caller MUST stop the server's container first (a live container writing to
// the volume during extraction would corrupt it).
func (r *Runtime) RestoreVolume(ctx context.Context, serverVol, id, file string) error {
	src := "/backups/" + id + "/" + file
	args := []string{
		"run", "--rm",
		"-v", serverVol + ":/data",
		"-v", backupsVolume + ":/backups",
		fileHelperImage, "sh", "-c",
		`[ -f "$1" ] || { echo "backup not found" >&2; exit 1; }; cd /data && rm -rf ./* ./.[!.]* ./..?* 2>/dev/null; tar xzf "$1" -C /data`,
		"_", src,
	}
	_, err := r.run(ctx, args...)
	return err
}

// DeleteBackup removes one backup archive.
func (r *Runtime) DeleteBackup(ctx context.Context, id, file string) error {
	args := []string{
		"run", "--rm", "-v", backupsVolume + ":/backups", fileHelperImage,
		"rm", "-f", "/backups/" + id + "/" + file,
	}
	_, err := r.run(ctx, args...)
	return err
}
