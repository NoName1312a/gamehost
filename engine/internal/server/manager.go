// Package server owns the lifecycle of game servers: it maps a game template
// plus the user's chosen settings onto a container spec, persists the server
// records to disk, and drives create/start/stop/delete via the runtime.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/templates"
)

// Server is a persisted game-server record. Live status is not stored; it's
// queried from the runtime on demand.
type Server struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	TemplateID    string               `json:"templateId"`
	Game          string               `json:"game"`
	Image         string               `json:"image"`
	Env           map[string]string    `json:"env"`
	Ports         []docker.PortMapping `json:"ports"`
	MemoryMB      int                  `json:"memoryMB"`
	DataPath      string               `json:"dataPath"`
	CommandMethod string               `json:"commandMethod"`
	CreatedAt     string               `json:"createdAt"`
	// RelayAddress is the public playit.gg address the user pasted back from
	// the playit dashboard for sharing this server. Empty if not using a relay.
	RelayAddress string `json:"relayAddress,omitempty"`
}

// ContainerName is the Docker container name for this server.
func (s *Server) ContainerName() string { return "gamehost-" + s.ID }

// VolumeName is the Docker named volume holding this server's data.
func (s *Server) VolumeName() string { return "gamehost-" + s.ID + "-data" }

// ServerView is a Server plus its live runtime status, returned to the UI.
type ServerView struct {
	*Server
	Status  string `json:"status"`
	Running bool   `json:"running"`
	// ExternalAddress is the public "host:port" friends connect to, when the
	// public IP is known (UPnP discovered). Shared is true when the primary
	// port is currently forwarded on the router.
	ExternalAddress string `json:"externalAddress,omitempty"`
	Shared          bool   `json:"shared"`
}

// CreateRequest is the payload to create a new server.
type CreateRequest struct {
	TemplateID string            `json:"templateId"`
	Name       string            `json:"name"`
	MemoryMB   int               `json:"memoryMB"`
	Port       int               `json:"port"` // primary host port
	Variables  map[string]string `json:"variables"`
}

// Runtime is the subset of the container runtime the manager needs.
// *docker.Runtime implements it; a future SDK-based runtime can too. Keeping it
// an interface also lets the manager be unit-tested without Docker.
type Runtime interface {
	Run(ctx context.Context, spec docker.CreateSpec) error
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	RemoveVolume(ctx context.Context, name string) error
	Inspect(ctx context.Context, name string) docker.State
}

// Networking is the subset of the UPnP port mapper the manager needs.
// *network.Mapper implements it. It's optional (may be nil) and every call is
// best-effort — a server never fails to start because forwarding failed.
type Networking interface {
	Map(ctx context.Context, port int, proto, desc string) error
	Unmap(ctx context.Context, port int, proto string) error
	ExternalIP() string
	IsMapped(port int, proto string) bool
}

// Manager is a concurrency-safe store of servers backed by a JSON file.
type Manager struct {
	mu    sync.RWMutex
	path  string
	rt    Runtime
	net   Networking // optional; nil disables auto port-forwarding
	reg   *templates.Registry
	items map[string]*Server
}

// NewManager creates the data dir, loads existing servers, and returns a
// Manager. net may be nil to disable UPnP auto-forwarding.
func NewManager(dataDir string, rt Runtime, net Networking, reg *templates.Registry) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	m := &Manager{
		path:  filepath.Join(dataDir, "servers.json"),
		rt:    rt,
		net:   net,
		reg:   reg,
		items: map[string]*Server{},
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) load() error {
	b, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var list []*Server
	if err := json.Unmarshal(b, &list); err != nil {
		return fmt.Errorf("parse %s: %w", m.path, err)
	}
	for _, s := range list {
		m.items[s.ID] = s
	}
	return nil
}

// save writes atomically; caller holds m.mu.
func (m *Manager) save() error {
	list := make([]*Server, 0, len(m.items))
	for _, s := range m.items {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt < list[j].CreatedAt })
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

func genID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Get returns a server record by ID.
func (m *Manager) Get(id string) (*Server, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.items[id]
	return s, ok
}

// SetRelayAddress stores the playit relay address the user shares for a server.
func (m *Manager) SetRelayAddress(id, addr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.items[id]
	if !ok {
		return fmt.Errorf("server not found")
	}
	s.RelayAddress = strings.TrimSpace(addr)
	return m.save()
}

// Create validates the request against its template and persists a new server.
// It does not launch the container — that happens on the first Start.
func (m *Manager) Create(req CreateRequest) (*Server, error) {
	t, ok := m.reg.Get(req.TemplateID)
	if !ok {
		return nil, fmt.Errorf("unknown template %q", req.TemplateID)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = t.Name
	}

	env := map[string]string{}
	for k, v := range t.Env {
		env[k] = v
	}
	for k, v := range req.Variables {
		if strings.TrimSpace(v) != "" {
			env[k] = v
		}
	}

	ports := make([]docker.PortMapping, 0, len(t.Ports))
	for i, p := range t.Ports {
		host := p.Default
		if i == 0 && req.Port > 0 {
			host = req.Port
		}
		if host < 1 || host > 65535 {
			return nil, fmt.Errorf("port %d is out of range (must be 1-65535)", host)
		}
		ports = append(ports, docker.PortMapping{Host: host, Container: p.Container, Protocol: p.Protocol})
	}

	mem := req.MemoryMB
	if mem <= 0 {
		mem = t.RecMemoryMB
	}
	if t.MinMemoryMB > 0 && mem < t.MinMemoryMB {
		return nil, fmt.Errorf("%s needs at least %d MB of memory", t.Name, t.MinMemoryMB)
	}
	dataPath := t.DataPath
	if dataPath == "" {
		dataPath = "/data"
	}

	s := &Server{
		ID:            genID(),
		Name:          name,
		TemplateID:    t.ID,
		Game:          t.Game,
		Image:         t.Image,
		Env:           env,
		Ports:         ports,
		MemoryMB:      mem,
		DataPath:      dataPath,
		CommandMethod: t.CommandMethod,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if owner := m.portOwner(ports); owner != "" {
		return nil, fmt.Errorf("port %d is already used by server %q", ports[0].Host, owner)
	}
	m.items[s.ID] = s
	if err := m.save(); err != nil {
		delete(m.items, s.ID)
		return nil, err
	}
	return s, nil
}

// portOwner returns the name of an existing server bound to any of the given
// host ports (same protocol), or "" if there's no conflict. Caller holds m.mu.
func (m *Manager) portOwner(ports []docker.PortMapping) string {
	return m.portOwnerExcept(ports, "")
}

// portOwnerExcept is portOwner but ignores the server with the given ID, so a
// server's own ports don't count as a conflict when updating it. Caller holds m.mu.
func (m *Manager) portOwnerExcept(ports []docker.PortMapping, exceptID string) string {
	for _, existing := range m.items {
		if existing.ID == exceptID {
			continue
		}
		for _, ep := range existing.Ports {
			for _, np := range ports {
				if ep.Host == np.Host && strings.EqualFold(ep.Protocol, np.Protocol) {
					return existing.Name
				}
			}
		}
	}
	return ""
}

// UpdateRequest changes an existing server's editable settings. Fields left at
// their zero value fall back to the server's current values.
type UpdateRequest struct {
	Name      string            `json:"name"`
	MemoryMB  int               `json:"memoryMB"`
	Port      int               `json:"port"` // primary host port
	Variables map[string]string `json:"variables"`
}

// Update validates and applies new settings to a server. Because a container's
// env/ports/memory are fixed at creation, it removes the existing container
// (keeping the data volume, so saved worlds/config survive) and lets the next
// Start recreate it from the new spec. If the server was running it is
// restarted so the change takes effect immediately.
func (m *Manager) Update(ctx context.Context, id string, req UpdateRequest) (*Server, error) {
	m.mu.Lock()
	s, ok := m.items[id]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("server not found")
	}
	t, ok := m.reg.Get(s.TemplateID)
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("unknown template %q", s.TemplateID)
	}

	// Compute (and validate) the new values before mutating anything.
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = s.Name
	}

	env := map[string]string{}
	for k, v := range t.Env {
		env[k] = v
	}
	for k, v := range req.Variables {
		if strings.TrimSpace(v) != "" {
			env[k] = v
		}
	}

	// Rebuild ports from the template, preserving the server's existing host
	// ports and letting req.Port override the primary one.
	ports := make([]docker.PortMapping, 0, len(t.Ports))
	for i, p := range t.Ports {
		host := p.Default
		if i < len(s.Ports) {
			host = s.Ports[i].Host
		}
		if i == 0 && req.Port > 0 {
			host = req.Port
		}
		if host < 1 || host > 65535 {
			m.mu.Unlock()
			return nil, fmt.Errorf("port %d is out of range (must be 1-65535)", host)
		}
		ports = append(ports, docker.PortMapping{Host: host, Container: p.Container, Protocol: p.Protocol})
	}

	mem := req.MemoryMB
	if mem <= 0 {
		mem = s.MemoryMB
	}
	if t.MinMemoryMB > 0 && mem < t.MinMemoryMB {
		m.mu.Unlock()
		return nil, fmt.Errorf("%s needs at least %d MB of memory", t.Name, t.MinMemoryMB)
	}

	if owner := m.portOwnerExcept(ports, id); owner != "" {
		m.mu.Unlock()
		return nil, fmt.Errorf("port %d is already used by server %q", ports[0].Host, owner)
	}
	m.mu.Unlock()

	// Tear down the old container (keep the data volume) so the new spec applies.
	st := m.rt.Inspect(ctx, s.ContainerName())
	wasRunning := st.Running
	if st.Running {
		_ = m.rt.Stop(ctx, s.ContainerName())
		m.unmapPorts(ctx, s) // unmap the OLD ports while s still holds them
	}
	if st.Exists {
		_ = m.rt.Remove(ctx, s.ContainerName())
	}

	m.mu.Lock()
	s.Name = name
	s.Env = env
	s.Ports = ports
	s.MemoryMB = mem
	if err := m.save(); err != nil {
		m.mu.Unlock()
		return nil, err
	}
	m.mu.Unlock()

	if wasRunning {
		if err := m.Start(ctx, id); err != nil {
			return s, fmt.Errorf("settings saved, but restarting the server failed: %w", err)
		}
	}
	return s, nil
}

func (m *Manager) specFor(s *Server) docker.CreateSpec {
	return docker.CreateSpec{
		Name:      s.ContainerName(),
		Image:     s.Image,
		Env:       s.Env,
		Ports:     s.Ports,
		MemoryMB:  s.MemoryMB,
		Volume:    s.VolumeName(),
		DataPath:  s.DataPath,
		OpenStdin: true,
	}
}

// Start launches the container (first time: docker run, which pulls the image)
// or starts the existing stopped container, then opens its port(s) on the
// router via UPnP (best-effort).
func (m *Manager) Start(ctx context.Context, id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("server not found")
	}
	var err error
	if m.rt.Inspect(ctx, s.ContainerName()).Exists {
		err = m.rt.Start(ctx, s.ContainerName())
	} else {
		err = m.rt.Run(ctx, m.specFor(s))
	}
	if err != nil {
		return err
	}
	m.mapPorts(ctx, s)
	return nil
}

// Stop stops the running container and closes its forwarded port(s).
func (m *Manager) Stop(ctx context.Context, id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("server not found")
	}
	err := m.rt.Stop(ctx, s.ContainerName())
	m.unmapPorts(ctx, s)
	return err
}

// Delete removes the container, its data volume, its port mappings, and the record.
func (m *Manager) Delete(ctx context.Context, id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("server not found")
	}
	m.unmapPorts(ctx, s)
	_ = m.rt.Remove(ctx, s.ContainerName())
	_ = m.rt.RemoveVolume(ctx, s.VolumeName())

	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, id)
	return m.save()
}

// mapPorts forwards each of the server's ports on the router. Best-effort: a
// missing/unsupported UPnP router is the common case and isn't an error.
func (m *Manager) mapPorts(ctx context.Context, s *Server) {
	if m.net == nil {
		return
	}
	for _, p := range s.Ports {
		if err := m.net.Map(ctx, p.Host, p.Protocol, "GameHost: "+s.Name); err != nil {
			slog.Debug("upnp map failed", "server", s.Name, "port", p.Host, "err", err)
		}
	}
}

func (m *Manager) unmapPorts(ctx context.Context, s *Server) {
	if m.net == nil {
		return
	}
	for _, p := range s.Ports {
		_ = m.net.Unmap(ctx, p.Host, p.Protocol)
	}
}

// List returns all servers with their live runtime status.
func (m *Manager) List(ctx context.Context) []ServerView {
	m.mu.RLock()
	servers := make([]*Server, 0, len(m.items))
	for _, s := range m.items {
		servers = append(servers, s)
	}
	m.mu.RUnlock()

	sort.Slice(servers, func(i, j int) bool { return servers[i].CreatedAt < servers[j].CreatedAt })

	views := make([]ServerView, 0, len(servers))
	for _, s := range servers {
		st := m.rt.Inspect(ctx, s.ContainerName())
		status := "not created"
		if st.Exists {
			status = st.Status
		}
		view := ServerView{Server: s, Status: status, Running: st.Running}
		// Surface connectivity for running servers once UPnP discovery has a
		// public IP / port mapping.
		if st.Running && m.net != nil && len(s.Ports) > 0 {
			primary := s.Ports[0]
			view.Shared = m.net.IsMapped(primary.Host, primary.Protocol)
			if ip := m.net.ExternalIP(); ip != "" {
				view.ExternalAddress = ip + ":" + strconv.Itoa(primary.Host)
			}
		}
		views = append(views, view)
	}
	return views
}
