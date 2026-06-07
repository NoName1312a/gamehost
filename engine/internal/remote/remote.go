// Package remote manages the engine's optional remote-access listener: an
// HTTPS server bound to a non-loopback address using a self-signed cert. It's
// off by default (desktop is loopback-only); enabling it is the Pro
// "manage from anywhere" feature. The shared auth middleware ensures remote
// callers must authenticate while local (loopback) ones still don't.
package remote

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/leop1/gamehost/engine/internal/tlscert"
)

// DefaultPort is the suggested remote-access HTTPS port.
const DefaultPort = 8788

// State is the persisted remote-access configuration plus the live bound addr.
type State struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Addr    string `json:"-"` // actual bound address while running, else ""
}

// Controller owns the remote-access setting and the HTTPS listener lifecycle.
type Controller struct {
	dataDir  string
	bindHost string
	path     string

	mu      sync.Mutex
	handler http.Handler
	srv     *http.Server
	ln      net.Listener
	state   State
}

// New loads any persisted state from <dataDir>/remote-access.json. bindHost
// defaults to 0.0.0.0 (all interfaces); tests pass 127.0.0.1.
func New(dataDir, bindHost string) *Controller {
	if bindHost == "" {
		bindHost = "0.0.0.0"
	}
	c := &Controller{dataDir: dataDir, bindHost: bindHost, path: filepath.Join(dataDir, "remote-access.json")}
	if b, err := os.ReadFile(c.path); err == nil {
		_ = json.Unmarshal(b, &c.state)
	}
	return c
}

// SetHandler sets the HTTP handler the remote listener serves (the API router).
func (c *Controller) SetHandler(h http.Handler) {
	c.mu.Lock()
	c.handler = h
	c.mu.Unlock()
}

// Status returns the current state, including the live bound address if running.
func (c *Controller) Status() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.state
	if c.ln != nil {
		s.Addr = c.ln.Addr().String()
	}
	return s
}

// Enable starts the HTTPS listener (idempotent). port>0 overrides the persisted
// port; port==0 reuses it (which may be 0 = ephemeral, used by tests).
func (c *Controller) Enable(port int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handler == nil {
		return errors.New("no handler set")
	}
	if c.srv != nil {
		return nil // already running
	}
	if port > 0 {
		c.state.Port = port
	}
	cert, err := tlscert.Ensure(c.dataDir)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(c.bindHost, strconv.Itoa(c.state.Port)))
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           c.handler,
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}},
		ReadHeaderTimeout: 10 * time.Second,
	}
	c.srv, c.ln = srv, ln
	c.state.Enabled = true
	c.persistLocked()
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	return nil
}

// Disable stops the listener and persists the off state.
func (c *Controller) Disable() error {
	c.mu.Lock()
	srv := c.srv
	c.srv, c.ln = nil, nil
	c.state.Enabled = false
	c.persistLocked()
	c.mu.Unlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
	return nil
}

// StartIfEnabled starts the listener at boot when remote access was left on.
func (c *Controller) StartIfEnabled() error {
	c.mu.Lock()
	enabled := c.state.Enabled
	c.mu.Unlock()
	if enabled {
		return c.Enable(0)
	}
	return nil
}

// Shutdown gracefully stops the listener (called on engine shutdown).
func (c *Controller) Shutdown(ctx context.Context) {
	c.mu.Lock()
	srv := c.srv
	c.srv, c.ln = nil, nil
	c.mu.Unlock()
	if srv != nil {
		_ = srv.Shutdown(ctx)
	}
}

// persistLocked writes the state file. Caller holds c.mu.
func (c *Controller) persistLocked() {
	b, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err == nil {
		_ = os.Rename(tmp, c.path)
	}
}
