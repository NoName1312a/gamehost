package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The installed app ships templates next to the engine binary (Tauri bundles
// them at <install>/resources/templates). The engine must find them from its
// own location — the working directory depends on how the OS launched us and
// the desktop shell's env plumbing must not be a single point of failure.
func TestDefaultTemplatesDirPrefersExeRelative(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable unavailable: %v", err)
	}
	want := filepath.Join(filepath.Dir(exe), "resources", "templates")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", want, err)
	}
	t.Cleanup(func() { os.RemoveAll(filepath.Join(filepath.Dir(exe), "resources")) })

	if got := defaultTemplatesDir(); got != want {
		t.Errorf("defaultTemplatesDir() = %q, want exe-relative %q", got, want)
	}
}
