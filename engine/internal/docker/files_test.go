package docker

import (
	"strings"
	"testing"
)

func TestContainerPathStaysUnderData(t *testing.T) {
	cases := map[string]string{
		"":                   "/data",
		"/":                  "/data",
		"server.properties":  "/data/server.properties",
		"config/foo.yml":     "/data/config/foo.yml",
		"/config/foo.yml":    "/data/config/foo.yml",
		"config\\bar.yml":    "/data/config/bar.yml", // backslashes normalised
		"../../etc/passwd":   "/data/etc/passwd",     // traversal collapsed
		"foo/../../bar":      "/data/bar",
		"../../../../":       "/data",
		"a/./b":              "/data/a/b",
	}
	for in, want := range cases {
		got := containerPath(in)
		if got != want {
			t.Errorf("containerPath(%q) = %q, want %q", in, got, want)
		}
		if !strings.HasPrefix(got, "/data") || strings.Contains(got, "..") {
			t.Errorf("containerPath(%q) = %q escaped the data root", in, got)
		}
	}
}
