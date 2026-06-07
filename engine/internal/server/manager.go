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
	// CPUs caps CPU cores for this server (e.g. 1.5). 0 leaves CPU uncapped.
	CPUs          float64 `json:"cpus,omitempty"`
	DataPath      string  `json:"dataPath"`
	CommandMethod string               `json:"commandMethod"`
	CreatedAt     string               `json:"createdAt"`
	// RelayAddress is the public playit.gg address the user pasted back from
	// the playit dashboard for sharing this server. Empty if not using a relay.
	RelayAddress string `json:"relayAddress,omitempty"`
	// RestartAt / BackupAt are optional daily schedule times in local "HH:MM"
	// (24h). Empty disables that schedule.
	RestartAt string `json:"restartAt,omitempty"`
	BackupAt  string `json:"backupAt,omitempty"`
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
	// Pulling/PullPercent/PullStatus reflect a first-start image download.
	Pulling     bool   `json:"pulling"`
	PullPercent int    `json:"pullPercent"`
	PullStatus  string `json:"pullStatus,omitempty"`
}

// CreateRequest is the payload to create a new server.
type CreateRequest struct {
	TemplateID string            `json:"templateId"`
	Name       string            `json:"name"`
	MemoryMB   int               `json:"memoryMB"`
	CPUs       float64           `json:"cpus"` // CPU-core cap; 0 = uncapped
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
	RestoreVolume(ctx context.Context, serverVol, id, file string) error
	CreateBackup(ctx context.Context, serverVol, id, file string) error
	ImageExists(ctx context.Context, image string) bool
	Pull(ctx context.Context, image string, onProgress func(percent int, status string)) error
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

// Relay is the subset of the playit relay agent the manager needs. *relay.Agent
// implements it. Optional (may be nil). The manager drives it so the agent runs
// only while a relay-shared server is actually hosting — not always-on.
type Relay interface {
	Start() error
	Stop()
}

// Manager is a concurrency-safe store of servers backed by a JSON file.
type Manager struct {
	mu    sync.RWMutex
	path  string
	rt    Runtime
	net   Networking // optional; nil disables auto port-forwarding
	relay Relay      // optional; nil disables relay lifecycle management
	reg   *templates.Registry
	items map[string]*Server

	pullMu sync.Mutex
	pulls  map[string]pullState // server id -> in-progress image download
}

// pullState is the live first-start image-download progress for a server.
type pullState struct {
	Percent int
	Status  string
}

// NewManager creates the data dir, loads existing servers, and returns a
// Manager. net and rel may be nil to disable UPnP auto-forwarding / relay
// supervision respectively.
func NewManager(dataDir string, rt Runtime, net Networking, rel Relay, reg *templates.Registry) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	m := &Manager{
		path:  filepath.Join(dataDir, "servers.json"),
		rt:    rt,
		net:   net,
		relay: rel,
		reg:   reg,
		items: map[string]*Server{},
		pulls: map[string]pullState{},
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

// currentSchemaVersion is bumped when the on-disk Server shape changes in a way
// that needs migration on load.
const currentSchemaVersion = 1

// fileFormat is the versioned on-disk wrapper for servers.json. Older installs
// stored a bare JSON array (no wrapper); readServers migrates those.
type fileFormat struct {
	SchemaVersion int       `json:"schemaVersion"`
	Servers       []*Server `json:"servers"`
}

// readServers parses a servers file in either the versioned wrapper format or
// the legacy v0 bare-array format. A missing file returns os.ErrNotExist.
func readServers(path string) ([]*Server, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Versioned format: {"schemaVersion":N,"servers":[...]}.
	var ff fileFormat
	if err := json.Unmarshal(b, &ff); err == nil && ff.SchemaVersion > 0 {
		return ff.Servers, nil
	}
	// Legacy v0 format: a bare JSON array.
	var list []*Server
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return list, nil
}

func (m *Manager) load() error {
	list, err := readServers(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		// The live file is corrupt/unreadable; try the .bak recovery copy
		// rather than refusing to boot (which would orphan running containers).
		bak, berr := readServers(m.path + ".bak")
		if berr != nil {
			return err // nothing to recover from; surface the original error
		}
		slog.Warn("servers.json unreadable; recovered from .bak", "err", err)
		list = bak
	}
	for _, s := range list {
		m.items[s.ID] = s
	}
	return nil
}

// save writes atomically; caller holds m.mu. It rotates the previous good file
// to <path>.bak first so a corrupt/partial write stays recoverable.
func (m *Manager) save() error {
	list := make([]*Server, 0, len(m.items))
	for _, s := range m.items {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt < list[j].CreatedAt })
	b, err := json.MarshalIndent(fileFormat{SchemaVersion: currentSchemaVersion, Servers: list}, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	// Best-effort: keep the last good file as .bak (skipped on the first save,
	// when no main file exists yet).
	if prev, rerr := os.ReadFile(m.path); rerr == nil {
		_ = os.WriteFile(m.path+".bak", prev, 0o644)
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
	s, ok := m.items[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("server not found")
	}
	s.RelayAddress = strings.TrimSpace(addr)
	err := m.save()
	m.mu.Unlock()
	m.syncRelay() // a server gaining/losing a relay address may flip agent state
	return err
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
	cpus := req.CPUs
	if cpus < 0 {
		cpus = 0
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
		CPUs:          cpus,
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
	CPUs      float64           `json:"cpus"` // CPU-core cap; 0 = keep current
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
	if req.CPUs > 0 {
		s.CPUs = req.CPUs
	}
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
		CPUs:      s.CPUs,
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
		// First start: pull the image with progress so the UI can show a bar,
		// then run (the image is now local, so Run won't pull again).
		if !m.rt.ImageExists(ctx, s.Image) {
			m.setPull(id, 0, "Preparing download…")
			perr := m.rt.Pull(ctx, s.Image, func(pct int, status string) { m.setPull(id, pct, status) })
			m.clearPull(id)
			if perr != nil {
				return perr
			}
		}
		err = m.rt.Run(ctx, m.specFor(s))
	}
	if err != nil {
		return err
	}
	m.mapPorts(ctx, s)
	m.syncRelay()
	return nil
}

func (m *Manager) setPull(id string, percent int, status string) {
	m.pullMu.Lock()
	m.pulls[id] = pullState{Percent: percent, Status: status}
	m.pullMu.Unlock()
}

func (m *Manager) clearPull(id string) {
	m.pullMu.Lock()
	delete(m.pulls, id)
	m.pullMu.Unlock()
}

func (m *Manager) getPull(id string) (pullState, bool) {
	m.pullMu.Lock()
	defer m.pullMu.Unlock()
	p, ok := m.pulls[id]
	return p, ok
}

// Stop stops the running container and closes its forwarded port(s).
func (m *Manager) Stop(ctx context.Context, id string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("server not found")
	}
	err := m.rt.Stop(ctx, s.ContainerName())
	m.unmapPorts(ctx, s)
	m.syncRelay()
	return err
}

// RestoreBackup restores a server's data volume from a backup archive. It stops
// the container first (extracting over a live volume would corrupt it) and
// restarts it afterward if it was running. The data volume is replaced; the
// container's settings (env/ports/memory) are unchanged.
func (m *Manager) RestoreBackup(ctx context.Context, id, file string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("server not found")
	}
	wasRunning := m.rt.Inspect(ctx, s.ContainerName()).Running
	if wasRunning {
		if err := m.Stop(ctx, id); err != nil {
			return fmt.Errorf("stop before restore failed: %w", err)
		}
	}
	if err := m.rt.RestoreVolume(ctx, s.VolumeName(), id, file); err != nil {
		return err
	}
	if wasRunning {
		if err := m.Start(ctx, id); err != nil {
			return fmt.Errorf("backup restored, but restarting the server failed: %w", err)
		}
	}
	return nil
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
	delete(m.items, id)
	err := m.save()
	m.mu.Unlock()
	m.syncRelay()
	return err
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

// syncRelay runs the playit relay agent iff at least one relay-shared server is
// currently running, and stops it otherwise — so it's never "always on", only
// up while you're actually hosting something shared through it. Best-effort.
func (m *Manager) syncRelay() {
	if m.relay == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if m.anyRelayServerRunning(ctx) {
		if err := m.relay.Start(); err != nil {
			slog.Debug("relay start skipped", "err", err)
		}
	} else {
		m.relay.Stop()
	}
}

// anyRelayServerRunning reports whether a server with a relay address has a
// running container.
func (m *Manager) anyRelayServerRunning(ctx context.Context) bool {
	m.mu.RLock()
	shared := make([]*Server, 0)
	for _, s := range m.items {
		if strings.TrimSpace(s.RelayAddress) != "" {
			shared = append(shared, s)
		}
	}
	m.mu.RUnlock()
	for _, s := range shared {
		if m.rt.Inspect(ctx, s.ContainerName()).Running {
			return true
		}
	}
	return false
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
		if p, ok := m.getPull(s.ID); ok {
			view.Pulling, view.PullPercent, view.PullStatus = true, p.Percent, p.Status
		}
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

// SetSchedule sets a server's daily restart/backup times ("HH:MM" 24h local, or
// "" to disable).
func (m *Manager) SetSchedule(id, restartAt, backupAt string) error {
	restartAt, backupAt = strings.TrimSpace(restartAt), strings.TrimSpace(backupAt)
	if !validHHMM(restartAt) || !validHHMM(backupAt) {
		return fmt.Errorf("times must be HH:MM (24-hour) or empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.items[id]
	if !ok {
		return fmt.Errorf("server not found")
	}
	s.RestartAt, s.BackupAt = restartAt, backupAt
	return m.save()
}

func validHHMM(s string) bool {
	if s == "" {
		return true
	}
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h, err1 := strconv.Atoi(s[:2])
	mn, err2 := strconv.Atoi(s[3:])
	return err1 == nil && err2 == nil && h >= 0 && h < 24 && mn >= 0 && mn < 60
}

// RunScheduler fires per-server daily restart/backup schedules until ctx is
// cancelled. It ticks every 30s (so each target minute is caught) and de-dupes
// by minute so an action fires at most once per scheduled time.
func (m *Manager) RunScheduler(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	fired := map[string]string{} // "r:"/"b:"+id -> last "2006-01-02 15:04" fired
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			hm, stamp := now.Format("15:04"), now.Format("2006-01-02 15:04")
			var restarts, backups []string
			m.mu.RLock()
			for _, s := range m.items {
				if s.RestartAt == hm && fired["r:"+s.ID] != stamp {
					restarts = append(restarts, s.ID)
				}
				if s.BackupAt == hm && fired["b:"+s.ID] != stamp {
					backups = append(backups, s.ID)
				}
			}
			m.mu.RUnlock()
			for _, id := range restarts {
				fired["r:"+id] = stamp
				safeGo("scheduled-restart:"+id, func() { m.scheduledRestart(id) })
			}
			for _, id := range backups {
				fired["b:"+id] = stamp
				safeGo("scheduled-backup:"+id, func() { m.scheduledBackup(id) })
			}
		}
	}
}

// guard runs fn, recovering and logging any panic so a background task can't
// take down the engine. Runs synchronously (use safeGo for a goroutine).
func guard(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("background task panicked", "task", name, "panic", r)
		}
	}()
	fn()
}

// safeGo runs fn in a goroutine under guard, so a panic in a scheduled task is
// logged instead of crashing the process.
func safeGo(name string, fn func()) { go guard(name, fn) }

func (m *Manager) scheduledRestart(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s, ok := m.Get(id)
	if !ok || !m.rt.Inspect(ctx, s.ContainerName()).Running {
		return // only restart a server that's actually running
	}
	slog.Info("scheduled restart", "server", s.Name)
	if err := m.Stop(ctx, id); err != nil {
		slog.Warn("scheduled restart: stop failed", "server", s.Name, "err", err)
		return
	}
	if err := m.Start(ctx, id); err != nil {
		slog.Warn("scheduled restart: start failed", "server", s.Name, "err", err)
	}
}

func (m *Manager) scheduledBackup(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s, ok := m.Get(id)
	if !ok {
		return
	}
	file := time.Now().UTC().Format("2006-01-02_15-04-05") + ".tar.gz"
	slog.Info("scheduled backup", "server", s.Name, "file", file)
	if err := m.rt.CreateBackup(ctx, s.VolumeName(), id, file); err != nil {
		slog.Warn("scheduled backup failed", "server", s.Name, "err", err)
	}
}
