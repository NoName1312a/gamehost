package telemetry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStoreDefaultsToDisabled(t *testing.T) {
	s := NewStore(t.TempDir())
	if s.IsEnabled() {
		t.Error("a fresh telemetry store must default to disabled (opt-in)")
	}
}

func TestSetEnabledPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.SetEnabled(true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if !s.IsEnabled() {
		t.Error("SetEnabled(true) should enable immediately")
	}
	if reopened := NewStore(dir); !reopened.IsEnabled() {
		t.Error("opt-in did not persist across reopen")
	}
}

func TestSetEnabledFalseDisablesAndPersists(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.SetEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := s.SetEnabled(false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if s.IsEnabled() {
		t.Error("SetEnabled(false) should disable")
	}
	if reopened := NewStore(dir); reopened.IsEnabled() {
		t.Error("disabled state did not persist across reopen")
	}
}

func TestCorruptSettingsFileFallsBackToDisabled(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "telemetry.json"), []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewStore(dir)
	if s.IsEnabled() {
		t.Error("a corrupt settings file must fall back to disabled, never silently enabled")
	}
}
