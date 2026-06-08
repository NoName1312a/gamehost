package server

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/templates"
)

// fakeRuntime implements Runtime without touching Docker. If running is non-nil
// it tracks per-container state (Run/Start/Stop update it, Inspect reads it);
// otherwise Inspect returns the static `state` (the simpler legacy behavior).
type fakeRuntime struct {
	state      docker.State
	running    map[string]bool
	lastSpec   docker.CreateSpec // captured by Run for spec assertions
	exportData []byte            // bytes ExportBackup streams out
}

func (f *fakeRuntime) Run(_ context.Context, spec docker.CreateSpec) error {
	f.lastSpec = spec
	if f.running != nil {
		f.running[spec.Name] = true
	}
	return nil
}
func (f *fakeRuntime) Start(_ context.Context, name string) error {
	if f.running != nil {
		f.running[name] = true
	}
	return nil
}
func (f *fakeRuntime) Stop(_ context.Context, name string) error {
	if f.running != nil {
		f.running[name] = false
	}
	return nil
}
func (f *fakeRuntime) Remove(_ context.Context, name string) error {
	if f.running != nil {
		delete(f.running, name)
	}
	return nil
}
func (f *fakeRuntime) RemoveVolume(context.Context, string) error { return nil }
func (f *fakeRuntime) Inspect(_ context.Context, name string) docker.State {
	if f.running != nil {
		running, exists := f.running[name]
		status := "exited"
		if running {
			status = "running"
		}
		return docker.State{Exists: exists, Running: running, Status: status}
	}
	return f.state
}
func (f *fakeRuntime) RestoreVolume(context.Context, string, string, string) error { return nil }
func (f *fakeRuntime) CreateBackup(context.Context, string, string, string) error  { return nil }
func (f *fakeRuntime) ExportBackup(_ context.Context, _, _ string, w io.Writer) error {
	_, err := w.Write(f.exportData)
	return err
}
func (f *fakeRuntime) ImageExists(context.Context, string) bool                    { return true }
func (f *fakeRuntime) Pull(context.Context, string, func(int, string)) error       { return nil }

// fakeRelay records start/stop calls so tests can assert the agent's lifecycle.
type fakeRelay struct {
	started bool
	stops   int
}

func (f *fakeRelay) Start() error { f.started = true; return nil }
func (f *fakeRelay) Stop()        { f.started = false; f.stops++ }

const testTemplate = `id: test-mc
name: Test MC
game: minecraft
category: Sandbox
image: itzg/minecraft-server:latest
runtime: java
dataPath: /data
commandMethod: rcon-cli
minMemoryMB: 1024
recMemoryMB: 4096
ports:
  - name: game
    container: 25565
    protocol: tcp
    default: 25565
env:
  EULA: "TRUE"
variables:
  - key: VERSION
    default: LATEST
`

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tdir, "test-mc.yaml"), []byte(testTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := templates.NewRegistry(tdir)
	if err := reg.Load(); err != nil {
		t.Fatalf("load templates: %v", err)
	}
	dataDir := t.TempDir()
	m, err := NewManager(dataDir, &fakeRuntime{}, nil, nil, reg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	return m, dataDir
}

func TestCreateMergesEnvAndDefaults(t *testing.T) {
	m, _ := newTestManager(t)
	s, err := m.Create(CreateRequest{
		TemplateID: "test-mc",
		Name:       "My MC",
		Port:       25565,
		Variables:  map[string]string{"VERSION": "1.21.4", "TYPE": "PAPER"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.Env["EULA"] != "TRUE" {
		t.Errorf("template env not merged: %v", s.Env)
	}
	if s.Env["VERSION"] != "1.21.4" || s.Env["TYPE"] != "PAPER" {
		t.Errorf("variables not applied: %v", s.Env)
	}
	if s.MemoryMB != 4096 {
		t.Errorf("memory should default to recommended 4096, got %d", s.MemoryMB)
	}
	if s.DataPath != "/data" || s.CommandMethod != "rcon-cli" {
		t.Errorf("template fields not carried: dataPath=%q cmd=%q", s.DataPath, s.CommandMethod)
	}
	if len(s.Ports) != 1 || s.Ports[0].Host != 25565 || s.Ports[0].Container != 25565 {
		t.Errorf("ports wrong: %+v", s.Ports)
	}
}

func TestCreateAppliesCPUsToSpec(t *testing.T) {
	tdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tdir, "test-mc.yaml"), []byte(testTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := templates.NewRegistry(tdir)
	if err := reg.Load(); err != nil {
		t.Fatalf("load templates: %v", err)
	}
	rt := &fakeRuntime{} // zero state => container "not created" => Start calls Run
	m, err := NewManager(t.TempDir(), rt, nil, nil, reg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	s, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "Capped", Port: 25565, CPUs: 2})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.CPUs != 2 {
		t.Errorf("CPUs not persisted on record: got %v want 2", s.CPUs)
	}
	if err := m.Start(context.Background(), s.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	if rt.lastSpec.CPUs != 2 {
		t.Errorf("CPUs not carried into container spec: got %v want 2", rt.lastSpec.CPUs)
	}
}

func TestCreateRejectsDuplicatePort(t *testing.T) {
	m, _ := newTestManager(t)
	if _, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "A", Port: 25565}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "B", Port: 25565})
	if err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("expected port-conflict error, got %v", err)
	}
}

func TestCreateRejectsUnknownTemplateAndLowMemory(t *testing.T) {
	m, _ := newTestManager(t)
	if _, err := m.Create(CreateRequest{TemplateID: "nope"}); err == nil {
		t.Error("expected error for unknown template")
	}
	_, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "Tiny", Port: 30000, MemoryMB: 256})
	if err == nil || !strings.Contains(err.Error(), "at least") {
		t.Fatalf("expected min-memory error, got %v", err)
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	m, dataDir := newTestManager(t)
	s, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "Persisted", Port: 25565})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// A fresh manager over the same dir should load the server back.
	reg := m.reg
	m2, err := NewManager(dataDir, &fakeRuntime{state: docker.State{Exists: true, Status: "running", Running: true}}, nil, nil, reg)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, ok := m2.Get(s.ID)
	if !ok || got.Name != "Persisted" {
		t.Fatalf("server not reloaded from disk: ok=%v got=%+v", ok, got)
	}

	views := m2.List(context.Background())
	if len(views) != 1 || views[0].Status != "running" || !views[0].Running {
		t.Fatalf("List did not reflect runtime status: %+v", views)
	}
}

func TestUpdateAppliesNewSettings(t *testing.T) {
	m, _ := newTestManager(t)
	s, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "Before", Port: 25565, Variables: map[string]string{"VERSION": "LATEST"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := m.Update(context.Background(), s.ID, UpdateRequest{
		Name:      "After",
		MemoryMB:  2048,
		Port:      25600,
		Variables: map[string]string{"VERSION": "1.21.4"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Name != "After" || got.MemoryMB != 2048 {
		t.Errorf("name/memory not updated: %+v", got)
	}
	if len(got.Ports) != 1 || got.Ports[0].Host != 25600 {
		t.Errorf("port not updated: %+v", got.Ports)
	}
	if got.Env["VERSION"] != "1.21.4" {
		t.Errorf("variable not updated: %v", got.Env)
	}
	if got.Env["EULA"] != "TRUE" {
		t.Errorf("template base env lost on update: %v", got.Env)
	}
	// A blank name keeps the current one; zero memory keeps the current one.
	got2, err := m.Update(context.Background(), s.ID, UpdateRequest{Port: 25600})
	if err != nil {
		t.Fatalf("update2: %v", err)
	}
	if got2.Name != "After" || got2.MemoryMB != 2048 {
		t.Errorf("zero-value fields should keep current values: %+v", got2)
	}
}

func TestUpdateRejectsConflictingPortAndLowMemory(t *testing.T) {
	m, _ := newTestManager(t)
	a, _ := m.Create(CreateRequest{TemplateID: "test-mc", Name: "A", Port: 25565})
	_, _ = m.Create(CreateRequest{TemplateID: "test-mc", Name: "B", Port: 25566})

	// Moving A onto B's port conflicts.
	if _, err := m.Update(context.Background(), a.ID, UpdateRequest{Port: 25566}); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("expected port-conflict error, got %v", err)
	}
	// Keeping A on its own port is fine (own port is not a conflict).
	if _, err := m.Update(context.Background(), a.ID, UpdateRequest{Port: 25565}); err != nil {
		t.Fatalf("updating to own port should be allowed: %v", err)
	}
	// Below-minimum memory is rejected.
	if _, err := m.Update(context.Background(), a.ID, UpdateRequest{MemoryMB: 256}); err == nil || !strings.Contains(err.Error(), "at least") {
		t.Fatalf("expected min-memory error, got %v", err)
	}
}

func TestRelayRunsOnlyWhileHosting(t *testing.T) {
	build := func(rt Runtime, rel Relay) *Manager {
		tdir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tdir, "test-mc.yaml"), []byte(testTemplate), 0o644); err != nil {
			t.Fatal(err)
		}
		reg := templates.NewRegistry(tdir)
		if err := reg.Load(); err != nil {
			t.Fatalf("load templates: %v", err)
		}
		m, err := NewManager(t.TempDir(), rt, nil, rel, reg)
		if err != nil {
			t.Fatalf("new manager: %v", err)
		}
		return m
	}

	// A running relay-shared server brings the agent up.
	rel := &fakeRelay{}
	m := build(&fakeRuntime{state: docker.State{Exists: true, Running: true, Status: "running"}}, rel)
	s, _ := m.Create(CreateRequest{TemplateID: "test-mc", Name: "A", Port: 25565})
	if err := m.SetRelayAddress(s.ID, "abc.playit.gg:30000"); err != nil {
		t.Fatalf("set relay: %v", err)
	}
	if !rel.started {
		t.Error("relay should be running while a relay-shared server is up")
	}

	// A server with no relay address must not start the agent, even when running.
	rel2 := &fakeRelay{}
	m2 := build(&fakeRuntime{state: docker.State{Exists: true, Running: true, Status: "running"}}, rel2)
	s2, _ := m2.Create(CreateRequest{TemplateID: "test-mc", Name: "B", Port: 25565})
	if err := m2.Start(context.Background(), s2.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	if rel2.started {
		t.Error("relay should not start for a server with no relay address")
	}

	// A relay-shared server that isn't running keeps the agent stopped.
	rel3 := &fakeRelay{}
	m3 := build(&fakeRuntime{state: docker.State{Exists: true, Running: false, Status: "exited"}}, rel3)
	s3, _ := m3.Create(CreateRequest{TemplateID: "test-mc", Name: "C", Port: 25565})
	if err := m3.SetRelayAddress(s3.ID, "abc.playit.gg:30000"); err != nil {
		t.Fatalf("set relay: %v", err)
	}
	if rel3.started || rel3.stops == 0 {
		t.Errorf("relay should be stopped when no relay-shared server is running (started=%v stops=%d)", rel3.started, rel3.stops)
	}
}

func TestValidHHMM(t *testing.T) {
	valid := []string{"", "00:00", "23:59", "09:30", "12:05"}
	invalid := []string{"9:30", "24:00", "12:60", "1234", "ab:cd", "12:5", "012:00"}
	for _, s := range valid {
		if !validHHMM(s) {
			t.Errorf("validHHMM(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if validHHMM(s) {
			t.Errorf("validHHMM(%q) = true, want false", s)
		}
	}
}

func testRegistry(t *testing.T) *templates.Registry {
	t.Helper()
	tdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tdir, "test-mc.yaml"), []byte(testTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := templates.NewRegistry(tdir)
	if err := reg.Load(); err != nil {
		t.Fatalf("load templates: %v", err)
	}
	return reg
}

// TestLoadMigratesLegacyArray verifies a v0 servers.json (a bare JSON array
// with no schemaVersion wrapper) still loads, so existing installs survive the
// format change.
func TestLoadMigratesLegacyArray(t *testing.T) {
	dataDir := t.TempDir()
	legacy := `[{"id":"abc","name":"Legacy","templateId":"test-mc","memoryMB":2048,"createdAt":"2024-01-01T00:00:00Z"}]`
	if err := os.WriteFile(filepath.Join(dataDir, "servers.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(dataDir, &fakeRuntime{}, nil, nil, testRegistry(t))
	if err != nil {
		t.Fatalf("new manager over legacy file: %v", err)
	}
	got, ok := m.Get("abc")
	if !ok || got.Name != "Legacy" {
		t.Fatalf("legacy server not migrated: ok=%v got=%+v", ok, got)
	}
}

// TestSaveWritesVersionedFormatAndBak verifies saves use the versioned wrapper
// and rotate a .bak recovery copy of the previous good file.
func TestSaveWritesVersionedFormatAndBak(t *testing.T) {
	dataDir := t.TempDir()
	m, err := NewManager(dataDir, &fakeRuntime{}, nil, nil, testRegistry(t))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if _, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "One", Port: 25565}); err != nil {
		t.Fatalf("create one: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dataDir, "servers.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"schemaVersion"`) {
		t.Errorf("servers.json is not in versioned format:\n%s", raw)
	}
	// A second mutation rotates the prior good file to servers.json.bak.
	if _, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "Two", Port: 25566}); err != nil {
		t.Fatalf("create two: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "servers.json.bak")); err != nil {
		t.Errorf("expected servers.json.bak after second save: %v", err)
	}
}

// TestLoadRecoversFromBak verifies a corrupt main file is recovered from the
// .bak copy instead of refusing to boot.
func TestLoadRecoversFromBak(t *testing.T) {
	dataDir := t.TempDir()
	reg := testRegistry(t)
	m, err := NewManager(dataDir, &fakeRuntime{}, nil, nil, reg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	one, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "One", Port: 25565})
	if err != nil {
		t.Fatalf("create one: %v", err)
	}
	two, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "Two", Port: 25566})
	if err != nil {
		t.Fatalf("create two: %v", err)
	}
	// Corrupt the live file; .bak holds the state before the 2nd save (just One).
	if err := os.WriteFile(filepath.Join(dataDir, "servers.json"), []byte("{ this is corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	m2, err := NewManager(dataDir, &fakeRuntime{}, nil, nil, reg)
	if err != nil {
		t.Fatalf("expected recovery from .bak, got error: %v", err)
	}
	if _, ok := m2.Get(one.ID); !ok {
		t.Errorf("recovered state missing server %q from .bak", one.ID)
	}
	if _, ok := m2.Get(two.ID); ok {
		t.Errorf("did not expect server %q (it was only in the corrupt main file)", two.ID)
	}
}

func TestFreeTierCapsRunningServers(t *testing.T) {
	reg := testRegistry(t)
	rt := &fakeRuntime{running: map[string]bool{}}
	m, err := NewManager(t.TempDir(), rt, nil, nil, reg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	m.SetEntitlement(func() bool { return false }) // free tier

	a, _ := m.Create(CreateRequest{TemplateID: "test-mc", Name: "A", Port: 25565})
	b, _ := m.Create(CreateRequest{TemplateID: "test-mc", Name: "B", Port: 25566})
	c, _ := m.Create(CreateRequest{TemplateID: "test-mc", Name: "C", Port: 25567})

	ctx := context.Background()
	if err := m.Start(ctx, a.ID); err != nil {
		t.Fatalf("start A: %v", err)
	}
	if err := m.Start(ctx, b.ID); err != nil {
		t.Fatalf("start B: %v", err)
	}
	if err := m.Start(ctx, c.ID); err == nil || !strings.Contains(err.Error(), "free plan") {
		t.Fatalf("3rd start on free tier should be blocked, got %v", err)
	}

	// Upgrading to Pro lifts the cap.
	m.SetEntitlement(func() bool { return true })
	if err := m.Start(ctx, c.ID); err != nil {
		t.Errorf("Pro should allow the 3rd server: %v", err)
	}
}

func TestFreeTierCannotSchedule(t *testing.T) {
	m, _ := newTestManager(t)
	m.SetEntitlement(func() bool { return false })
	s, _ := m.Create(CreateRequest{TemplateID: "test-mc", Name: "S", Port: 25565})

	if err := m.SetSchedule(s.ID, "03:00", ""); err == nil || !strings.Contains(err.Error(), "Pro") {
		t.Fatalf("free tier should not set a schedule, got %v", err)
	}
	// Clearing a schedule is always allowed.
	if err := m.SetSchedule(s.ID, "", ""); err != nil {
		t.Errorf("clearing a schedule should be allowed on free: %v", err)
	}
	// Pro can schedule.
	m.SetEntitlement(func() bool { return true })
	if err := m.SetSchedule(s.ID, "03:00", "02:00"); err != nil {
		t.Errorf("Pro should be able to schedule: %v", err)
	}
}

func TestBackupCopiesOffsiteOnlyWhenProAndConfigured(t *testing.T) {
	reg := testRegistry(t)
	rt := &fakeRuntime{exportData: []byte("ARCHIVE-BYTES")}
	dataDir := t.TempDir()
	m, err := NewManager(dataDir, rt, nil, nil, reg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	dest := t.TempDir()
	if err := m.SetOffsiteDir(dest); err != nil {
		t.Fatalf("set offsite dir: %v", err)
	}
	s, _ := m.Create(CreateRequest{TemplateID: "test-mc", Name: "S", Port: 25565})
	ctx := context.Background()

	// Free tier: a backup is created but NOT copied off-site.
	m.SetEntitlement(func() bool { return false })
	file, err := m.Backup(ctx, s.ID)
	if err != nil {
		t.Fatalf("free backup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, file)); err == nil {
		t.Error("free tier must not copy backups off-site")
	}

	// Pro tier: the archive is copied to the off-site folder.
	m.SetEntitlement(func() bool { return true })
	file, err = m.Backup(ctx, s.ID)
	if err != nil {
		t.Fatalf("pro backup: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dest, file))
	if err != nil {
		t.Fatalf("off-site copy missing: %v", err)
	}
	if string(got) != "ARCHIVE-BYTES" {
		t.Errorf("off-site copy content = %q, want ARCHIVE-BYTES", got)
	}
}

func TestDelete(t *testing.T) {
	m, _ := newTestManager(t)
	s, _ := m.Create(CreateRequest{TemplateID: "test-mc", Name: "Doomed", Port: 25565})
	if err := m.Delete(context.Background(), s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := m.Get(s.ID); ok {
		t.Error("server still present after delete")
	}
}
