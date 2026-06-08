package offsite

import (
	"path/filepath"
	"testing"
)

func TestSetDirPersistsAndValidates(t *testing.T) {
	data := t.TempDir()
	target := t.TempDir()

	s := New(data)
	if s.Dir() != "" {
		t.Fatal("fresh store should have no offsite dir")
	}
	if err := s.SetDir(target); err != nil {
		t.Fatalf("set dir: %v", err)
	}
	if s.Dir() != target {
		t.Errorf("Dir() = %q, want %q", s.Dir(), target)
	}
	// Persists across reopen.
	if s2 := New(data); s2.Dir() != target {
		t.Errorf("offsite dir did not persist: %q", s2.Dir())
	}
	// Clearing is allowed.
	if err := s.SetDir(""); err != nil {
		t.Errorf("clearing should be allowed: %v", err)
	}
	if s.Dir() != "" {
		t.Error("Dir() should be empty after clear")
	}
}

func TestSetDirRejectsMissingFolder(t *testing.T) {
	s := New(t.TempDir())
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if err := s.SetDir(missing); err == nil {
		t.Error("setting a non-existent folder must fail")
	}
}
