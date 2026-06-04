package docker

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Stats is a one-shot resource sample for a running container.
type Stats struct {
	CPUPercent float64 `json:"cpuPercent"`
	MemUsedMB  float64 `json:"memUsedMB"`
	MemLimitMB float64 `json:"memLimitMB"`
	MemPercent float64 `json:"memPercent"`
}

// Stats samples a running container's CPU/memory usage via `docker stats`.
func (r *Runtime) Stats(ctx context.Context, name string) (Stats, error) {
	out, err := r.run(ctx, "stats", "--no-stream", "--format",
		"{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}", name)
	if err != nil {
		return Stats{}, err
	}
	parts := strings.Split(strings.TrimSpace(out), "\t")
	if len(parts) != 3 {
		return Stats{}, fmt.Errorf("unexpected stats output: %q", out)
	}
	used, limit := parseMemUsage(parts[1])
	return Stats{
		CPUPercent: parsePercent(parts[0]),
		MemUsedMB:  used,
		MemLimitMB: limit,
		MemPercent: parsePercent(parts[2]),
	}, nil
}

func parsePercent(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "%"), 64)
	return v
}

// parseMemUsage parses docker's "512.3MiB / 3.844GiB" into used/limit megabytes.
func parseMemUsage(s string) (used, limit float64) {
	a, b, ok := strings.Cut(s, "/")
	if !ok {
		return 0, 0
	}
	return parseSizeMB(a), parseSizeMB(b)
}

// parseSizeMB converts a docker size string (e.g. "512MiB", "3.8GiB", "900kB")
// to megabytes.
func parseSizeMB(s string) float64 {
	s = strings.TrimSpace(s)
	units := []struct {
		suffix string
		toMB   float64
	}{
		{"GiB", 1024}, {"MiB", 1}, {"KiB", 1.0 / 1024}, {"GB", 1000}, {"MB", 1},
		{"kB", 1.0 / 1000}, {"KB", 1.0 / 1000}, {"B", 1.0 / (1024 * 1024)},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			n, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, u.suffix)), 64)
			return n * u.toMB
		}
	}
	return 0
}
