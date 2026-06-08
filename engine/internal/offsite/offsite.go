// Package offsite stores the optional folder that backups are copied to for
// off-site safekeeping (a NAS, external drive, or a synced cloud folder like
// OneDrive/Dropbox). It's just a persisted path — the copy itself is done by the
// server manager. This is a Pro feature.
package offsite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store persists the off-site backup folder.
type Store struct {
	path string
	mu   sync.RWMutex
	dir  string
}

type state struct {
	Dir string `json:"dir"`
}

// New loads any persisted folder from <dataDir>/offsite.json.
func New(dataDir string) *Store {
	s := &Store{path: filepath.Join(dataDir, "offsite.json")}
	if b, err := os.ReadFile(s.path); err == nil {
		var st state
		if json.Unmarshal(b, &st) == nil {
			s.dir = st.Dir
		}
	}
	return s
}

// Dir returns the configured off-site folder, or "" if none.
func (s *Store) Dir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dir
}

// SetDir validates (it must exist) and persists the off-site folder. An empty
// string clears it.
func (s *Store) SetDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir != "" {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("that folder doesn't exist or isn't a directory")
		}
	}
	b, err := json.MarshalIndent(state{Dir: dir}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, b, 0o644); err != nil {
		return err
	}
	s.mu.Lock()
	s.dir = dir
	s.mu.Unlock()
	return nil
}
