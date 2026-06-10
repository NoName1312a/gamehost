package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/leop1/gamehost/engine/internal/templates"
)

func TestPurgeAllRemovesAndPersists(t *testing.T) {
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
	ctx := context.Background()
	if _, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "Alpha", Port: 25565}); err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	if _, err := m.Create(CreateRequest{TemplateID: "test-mc", Name: "Bravo", Port: 25566}); err != nil {
		t.Fatalf("create bravo: %v", err)
	}

	n, err := m.PurgeAll(ctx)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 2 {
		t.Errorf("removed count: got %d, want 2", n)
	}
	if got := len(m.List(ctx)); got != 0 {
		t.Errorf("after purge: got %d servers in memory, want 0", got)
	}

	// Purge must persist: a fresh manager over the same data dir sees nothing.
	m2, err := NewManager(dataDir, &fakeRuntime{}, nil, nil, reg)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := len(m2.List(ctx)); got != 0 {
		t.Errorf("purge did not persist: reopened manager has %d servers, want 0", got)
	}
}

func TestPurgeAllOnEmptyManagerIsZero(t *testing.T) {
	m, _ := newTestManager(t)
	n, err := m.PurgeAll(context.Background())
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 0 {
		t.Errorf("empty purge: got %d, want 0", n)
	}
}
