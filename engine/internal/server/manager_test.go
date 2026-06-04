package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/templates"
)

// fakeRuntime implements Runtime without touching Docker.
type fakeRuntime struct {
	state docker.State
}

func (f *fakeRuntime) Run(context.Context, docker.CreateSpec) error  { return nil }
func (f *fakeRuntime) Start(context.Context, string) error           { return nil }
func (f *fakeRuntime) Stop(context.Context, string) error            { return nil }
func (f *fakeRuntime) Remove(context.Context, string) error          { return nil }
func (f *fakeRuntime) RemoveVolume(context.Context, string) error    { return nil }
func (f *fakeRuntime) Inspect(context.Context, string) docker.State  { return f.state }

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
	m, err := NewManager(dataDir, &fakeRuntime{}, nil, reg)
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
	m2, err := NewManager(dataDir, &fakeRuntime{state: docker.State{Exists: true, Status: "running", Running: true}}, nil, reg)
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
