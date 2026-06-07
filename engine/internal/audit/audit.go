// Package audit records an append-only log of mutating actions (who/what/when)
// so a networked deployment has an accountability trail. It's intentionally
// minimal: one JSON object per line.
package audit

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger writes audit entries as JSON lines to an underlying writer.
type Logger struct {
	mu sync.Mutex
	w  io.Writer
}

// New returns a Logger writing to w.
func New(w io.Writer) *Logger { return &Logger{w: w} }

// NewFile opens (append, create) <dataDir>/audit.log for the engine's lifetime.
func NewFile(dataDir string) (*Logger, error) {
	f, err := os.OpenFile(filepath.Join(dataDir, "audit.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &Logger{w: f}, nil
}

type entry struct {
	Time   string `json:"time"`
	Actor  string `json:"actor"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
}

// Record appends one audit line. A nil Logger is a no-op, so audit is optional.
func (l *Logger) Record(actor, method, path string, status int) {
	if l == nil {
		return
	}
	b, err := json.Marshal(entry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		Actor:  actor,
		Method: method,
		Path:   path,
		Status: status,
	})
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.w.Write(append(b, '\n'))
}
