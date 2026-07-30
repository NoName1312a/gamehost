package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A failed load used to leave no trace outside a log line, so the app showed
// "no games" — the same thing an empty directory shows. That ambiguity is how
// the 0.6.6 empty-library bug shipped: the engine was reading the wrong
// directory and nothing said so.
func TestLoadStateReportsAMissingDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	r := NewRegistry(missing)

	if err := r.Load(); err == nil {
		t.Fatal("Load should fail for a missing directory")
	}

	st := r.LoadState()
	if st.Error == "" {
		t.Error("LoadState reports no error, so a broken load looks like an empty catalogue")
	}
	if st.Dir != missing {
		t.Errorf("LoadState.Dir = %q, want %q — the path is the thing that is usually wrong", st.Dir, missing)
	}
	if st.Count != 0 {
		t.Errorf("LoadState.Count = %d, want 0", st.Count)
	}
}

func TestLoadStateReportsSuccess(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "mc.yaml", "id: mc\nname: Minecraft\n")
	write(t, dir, "valheim.yml", "id: valheim\nname: Valheim\n")

	r := NewRegistry(dir)
	if err := r.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	st := r.LoadState()
	if st.Error != "" {
		t.Errorf("LoadState.Error = %q, want empty", st.Error)
	}
	if st.Count != 2 {
		t.Errorf("LoadState.Count = %d, want 2", st.Count)
	}
	if st.Dir != dir {
		t.Errorf("LoadState.Dir = %q, want %q", st.Dir, dir)
	}
}

// An empty but readable directory is a real state, distinct from a failure.
func TestLoadStateDistinguishesEmptyFromBroken(t *testing.T) {
	r := NewRegistry(t.TempDir())
	if err := r.Load(); err != nil {
		t.Fatalf("Load on an empty dir should succeed: %v", err)
	}

	st := r.LoadState()
	if st.Error != "" {
		t.Errorf("empty directory reported as an error: %q", st.Error)
	}
	if st.Count != 0 {
		t.Errorf("LoadState.Count = %d, want 0", st.Count)
	}
}

// A recovered load must clear the previous failure.
func TestLoadStateClearsAfterARecoveredLoad(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "broken.yaml", "id: [this is not valid yaml\n")

	r := NewRegistry(dir)
	if err := r.Load(); err == nil {
		t.Fatal("Load should fail on malformed yaml")
	}
	if !strings.Contains(r.LoadState().Error, "broken.yaml") {
		t.Errorf("error should name the offending file, got %q", r.LoadState().Error)
	}

	if err := os.Remove(filepath.Join(dir, "broken.yaml")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	write(t, dir, "mc.yaml", "id: mc\nname: Minecraft\n")

	if err := r.Load(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if st := r.LoadState(); st.Error != "" || st.Count != 1 {
		t.Errorf("stale failure survived a good reload: %+v", st)
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
