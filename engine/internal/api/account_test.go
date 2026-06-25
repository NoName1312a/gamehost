package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/leop1/gamehost/engine/internal/account"
	"github.com/leop1/gamehost/engine/internal/audit"
	"github.com/leop1/gamehost/engine/internal/auth"
	"github.com/leop1/gamehost/engine/internal/config"
	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/license"
	"github.com/leop1/gamehost/engine/internal/remote"
	"github.com/leop1/gamehost/engine/internal/server"
	"github.com/leop1/gamehost/engine/internal/telemetry"
	"github.com/leop1/gamehost/engine/internal/templates"
)

// buildRouterWithAccount constructs a real router with an optional account.Store
// injected into Deps. Mirrors the newTestAPIFull helper but adds Account.
func buildRouterWithAccount(t *testing.T, acct *account.Store) (http.Handler, *server.Manager) {
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
	mgr, err := server.NewManager(t.TempDir(), rt, nil, reg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if acct != nil {
		mgr.SetAccount(AdaptAccount(acct))
	}
	au, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatalf("new auth: %v", err)
	}
	rc := remote.New(t.TempDir(), "127.0.0.1")
	lic := license.NewStore(t.TempDir(), license.EmbeddedPublicKey())
	var auditBuf bytes.Buffer
	cfg := config.Config{AllowOrigins: []string{"http://localhost:5173"}}
	h := NewRouter(Deps{
		Cfg:       cfg,
		RT:        rt,
		Reg:       reg,
		Mgr:       mgr,
		Auth:      au,
		Remote:    rc,
		Audit:     audit.New(&auditBuf),
		License:   lic,
		Telemetry: telemetry.NewStore(t.TempDir()),
		Account:   acct,
	})
	return h, mgr
}

// fakePlatformServer returns an httptest.Server that acts as the GameNest
// platform: POST /api/link accepts code "GOODCODE" and returns a device token;
// rejects any other code with 400.
func fakePlatformServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/link":
			var b map[string]string
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["code"] != "GOODCODE" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "bad code"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"deviceToken": "dev-tok-test"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestAccountStatusNotConfigured verifies that GET /api/account reports
// configured=false when no account.Store is wired (GAMENEST_PLATFORM_URL unset).
func TestAccountStatusNotConfigured(t *testing.T) {
	h, _, _ := newTestAPI(t) // Deps.Account is nil
	rec := do(t, h, http.MethodGet, "/api/account", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["configured"] != false {
		t.Fatalf("nil account should report configured=false, got %v", got)
	}
}

// TestAccountLinkNotConfigured verifies that POST /api/account/link returns 503
// when no account.Store is wired.
func TestAccountLinkNotConfigured(t *testing.T) {
	h, _, _ := newTestAPI(t)
	rec := do(t, h, http.MethodPost, "/api/account/link", `{"code":"GOODCODE"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// TestAccountUnlinkNotConfigured verifies that DELETE /api/account/link returns
// 503 when no account.Store is wired.
func TestAccountUnlinkNotConfigured(t *testing.T) {
	h, _, _ := newTestAPI(t)
	rec := do(t, h, http.MethodDelete, "/api/account/link", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// TestAccountLinkFlow exercises link → status → unlink against a fake platform.
func TestAccountLinkFlow(t *testing.T) {
	plat := fakePlatformServer(t)
	acct := account.New(t.TempDir(), plat.URL)
	h, _ := buildRouterWithAccount(t, acct)

	// GET before linking: configured=true, linked=false
	rec := do(t, h, http.MethodGet, "/api/account", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get account: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status["configured"] != true {
		t.Fatalf("want configured=true when account store is wired, got %v", status["configured"])
	}
	if status["linked"] != false {
		t.Fatalf("want linked=false before linking, got %v", status["linked"])
	}

	// POST with empty code → 400
	rec = do(t, h, http.MethodPost, "/api/account/link", `{"code":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty code: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}

	// POST with bad code → 400 (platform rejects it)
	rec = do(t, h, http.MethodPost, "/api/account/link", `{"code":"BADCODE"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad code: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}

	// POST with good code → 200
	rec = do(t, h, http.MethodPost, "/api/account/link", `{"code":"GOODCODE"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("good code: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	// GET after linking: linked=true
	rec = do(t, h, http.MethodGet, "/api/account", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status["linked"] != true {
		t.Fatalf("want linked=true after linking, got %v", status["linked"])
	}

	// DELETE → 200
	rec = do(t, h, http.MethodDelete, "/api/account/link", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("unlink: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	// GET after unlink: linked=false
	rec = do(t, h, http.MethodGet, "/api/account", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status["linked"] != false {
		t.Fatalf("want linked=false after unlink, got %v", status["linked"])
	}
}

// TestSetVanitySlugValid verifies that a valid lowercase slug is accepted and
// an empty name reverts to auto.
func TestSetVanitySlugValid(t *testing.T) {
	h, mgr := buildRouterWithAccount(t, nil)
	s, err := mgr.Create(server.CreateRequest{TemplateID: "test-mc", Name: "VanityTest", Port: 25565})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	rec := do(t, h, http.MethodPut, "/api/servers/"+s.ID+"/vanity", `{"name":"alice"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid vanity name: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	rec = do(t, h, http.MethodPut, "/api/servers/"+s.ID+"/vanity", `{"name":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("empty vanity (revert to auto): want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// TestSetVanitySlugInvalid verifies that the gn- reserved prefix and other
// invalid patterns are rejected with 400.
func TestSetVanitySlugInvalid(t *testing.T) {
	h, mgr := buildRouterWithAccount(t, nil)
	s, err := mgr.Create(server.CreateRequest{TemplateID: "test-mc", Name: "VanityInvalid", Port: 25565})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// gn- prefix is reserved
	rec := do(t, h, http.MethodPut, "/api/servers/"+s.ID+"/vanity", `{"name":"gn-x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("gn- prefix: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}

	// Uppercase letters not allowed
	rec = do(t, h, http.MethodPut, "/api/servers/"+s.ID+"/vanity", `{"name":"BadName"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("uppercase: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}
