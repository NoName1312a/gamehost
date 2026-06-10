// Package telemetry implements opt-in, off-by-default diagnostics: a persisted
// consent setting plus a best-effort reporter for crash and usage events. No
// data ever leaves the machine unless the user has explicitly opted in AND a
// telemetry endpoint is configured.
package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// settingsVersion lets us migrate the on-disk format later without misreading
// an older file as "enabled".
const settingsVersion = 1

// settings is the on-disk shape of the consent file.
type settings struct {
	Enabled bool `json:"enabled"`
	Version int  `json:"version"`
}

// Store holds the user's telemetry consent, persisted to <dataDir>/telemetry.json.
// It is safe for concurrent use.
type Store struct {
	path string
	mu   sync.RWMutex
	s    settings
}

// NewStore loads consent from <dataDir>/telemetry.json. A missing, unreadable,
// or corrupt file yields the safe default: disabled (opt-in).
func NewStore(dataDir string) *Store {
	st := &Store{
		path: filepath.Join(dataDir, "telemetry.json"),
		s:    settings{Enabled: false, Version: settingsVersion},
	}
	if b, err := os.ReadFile(st.path); err == nil {
		var loaded settings
		if err := json.Unmarshal(b, &loaded); err == nil && loaded.Version == settingsVersion {
			st.s = loaded
		}
	}
	return st
}

// IsEnabled reports whether the user has opted in.
func (s *Store) IsEnabled() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.s.Enabled
}

// SetEnabled records and persists the consent choice atomically.
func (s *Store) SetEnabled(enabled bool) error {
	s.mu.Lock()
	s.s = settings{Enabled: enabled, Version: settingsVersion}
	b, err := json.MarshalIndent(s.s, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
