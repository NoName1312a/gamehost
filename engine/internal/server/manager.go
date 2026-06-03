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
	"os"
	"path/filepath"
	"sort"
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
}

// CreateRequest is the payload to create a new server.
type CreateRequest struct {
	TemplateID string            `json:"templateId"`
	Name       string            `json:"name"`
	MemoryMB   int               `json:"memoryMB"`
	Port       int               `json:"port"` // primary host port
	Variables  map[string]string `json:"variables"`
}

// Manager is a concurrency-safe store of servers backed by a JSON file.
type Manager struct {
	mu    sync.RWMutex
	path  string
	rt    *docker.Runtime
	reg   *templates.Registry
	items map[string]*Server
}

// NewManager creates the data dir, loads existing servers, and returns a Manager.
func NewManager(dataDir string, rt *docker.Runtime, reg *templates.Registry) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	m := &Manager{
		path:  filepath.Join(dataDir, "servers.json"),
		rt:    rt,
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
		ports = append(ports, docker.PortMapping{Host: host, Container: p.Container, Protocol: p.Protocol})
	}

	mem := req.MemoryMB
	if mem <= 0 {
		mem = t.RecMemoryMB
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
	m.items[s.ID] = s
	if err := m.save(); err != nil {
		delete(m.items, s.ID)
		return nil, err
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
// or starts the existing stopped container.
func (m *Manager) Start(ctx context.Context, id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("server not found")
	}
	if m.rt.Inspect(ctx, s.ContainerName()).Exists {
		return m.rt.Start(ctx, s.ContainerName())
	}
	return m.rt.Run(ctx, m.specFor(s))
}

// Stop stops the running container.
func (m *Manager) Stop(ctx context.Context, id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("server not found")
	}
	return m.rt.Stop(ctx, s.ContainerName())
}

// Delete removes the container, its data volume, and the record.
func (m *Manager) Delete(ctx context.Context, id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("server not found")
	}
	_ = m.rt.Remove(ctx, s.ContainerName())
	_ = m.rt.RemoveVolume(ctx, s.VolumeName())

	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, id)
	return m.save()
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
		views = append(views, ServerView{Server: s, Status: status, Running: st.Running})
	}
	return views
}
