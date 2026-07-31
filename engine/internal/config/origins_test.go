package config

import (
	"slices"
	"strings"
	"testing"
)

// A released build used to trust http://localhost:5173 — a port anything on the
// machine can bind. The host guard and the CSRF header still stood in the way,
// so it was never a hole by itself, but a shipped binary has no business
// trusting a dev server it will never talk to.
func TestReleaseBuildDoesNotTrustTheDevServer(t *testing.T) {
	t.Setenv("GAMEHOST_DEV", "")

	for _, o := range Load().AllowOrigins {
		if strings.Contains(o, "5173") {
			t.Errorf("release build allows the Vite dev origin %q", o)
		}
	}
}

func TestDevBuildTrustsTheDevServer(t *testing.T) {
	t.Setenv("GAMEHOST_DEV", "1")

	origins := Load().AllowOrigins
	for _, want := range []string{"http://localhost:5173", "http://127.0.0.1:5173"} {
		if !slices.Contains(origins, want) {
			t.Errorf("GAMEHOST_DEV=1 should allow %q; got %v", want, origins)
		}
	}
}

// The desktop webview is the only origin that matters in a shipped build; it
// must be trusted whether or not the dev flag is set.
func TestTauriOriginsAreAlwaysTrusted(t *testing.T) {
	for _, dev := range []string{"", "1"} {
		t.Setenv("GAMEHOST_DEV", dev)

		origins := Load().AllowOrigins
		for _, want := range []string{"http://tauri.localhost", "https://tauri.localhost", "tauri://localhost"} {
			if !slices.Contains(origins, want) {
				t.Errorf("GAMEHOST_DEV=%q: missing %q from %v", dev, want, origins)
			}
		}
	}
}
