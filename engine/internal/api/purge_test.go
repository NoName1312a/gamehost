package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/leop1/gamehost/engine/internal/server"
)

func TestPurgeRemovesAllServers(t *testing.T) {
	h, mgr, _ := newTestAPI(t)
	if _, err := mgr.Create(server.CreateRequest{TemplateID: "test-mc", Name: "A", Port: 25565}); err != nil {
		t.Fatalf("create A: %v", err)
	}
	if _, err := mgr.Create(server.CreateRequest{TemplateID: "test-mc", Name: "B", Port: 25566}); err != nil {
		t.Fatalf("create B: %v", err)
	}

	rec := do(t, h, http.MethodPost, "/api/system/purge", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"removed":2`) {
		t.Errorf("expected removed:2, got %s", rec.Body.String())
	}
}
