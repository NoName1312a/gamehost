package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leop1/gamehost/engine/internal/auth"
	"github.com/leop1/gamehost/engine/internal/config"
	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/server"
	"github.com/leop1/gamehost/engine/internal/templates"
)

const apiTestTemplate = `id: test-mc
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
`

// newTestAPI builds the real router over a docker-less runtime and an empty data
// dir, plus the manager so tests can seed a server. Handlers that actually shell
// out to Docker will fail, but request-level behavior (validation, body caps,
// routing) is exercised end-to-end through the real chi router. The auth store
// is returned so tests can exercise login/session behavior.
func newTestAPI(t *testing.T) (http.Handler, *server.Manager, *auth.Store) {
	t.Helper()
	tdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tdir, "test-mc.yaml"), []byte(apiTestTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := templates.NewRegistry(tdir)
	if err := reg.Load(); err != nil {
		t.Fatalf("load templates: %v", err)
	}
	rt := docker.New()
	mgr, err := server.NewManager(t.TempDir(), rt, nil, nil, reg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	au, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatalf("new auth: %v", err)
	}
	cfg := config.Config{AllowOrigins: []string{"http://localhost:5173"}}
	return NewRouter(cfg, rt, reg, mgr, nil, nil, au), mgr, au
}

// TestWriteFileRejectsOversizedBody verifies the file-write endpoint caps the
// request body, so a huge payload can't exhaust engine memory (413, not 400/200).
func TestWriteFileRejectsOversizedBody(t *testing.T) {
	h, mgr, _ := newTestAPI(t)
	s, err := mgr.Create(server.CreateRequest{TemplateID: "test-mc", Name: "X", Port: 25565})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	huge := strings.Repeat("a", maxFileWriteBytes+1024)
	body := `{"path":"server.properties","content":"` + huge + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/servers/"+s.ID+"/files", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:50000" // loopback: the desktop case
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized write: want 413, got %d (%s)", rec.Code, rec.Body.String())
	}
}
