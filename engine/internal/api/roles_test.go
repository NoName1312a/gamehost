package api

import (
	"net/http"
	"testing"

	"github.com/leop1/gamehost/engine/internal/auth"
	"github.com/leop1/gamehost/engine/internal/server"
)

// operatorSession logs in as a freshly created operator account and returns its
// session cookie — the audit's scenario: remote access is on and an
// admin/operator account has been handed to someone else.
func operatorSession(t *testing.T, h http.Handler, au *auth.Store, role string) *http.Cookie {
	t.Helper()
	if err := au.SetPassword("ownerpass123"); err != nil {
		t.Fatalf("set owner password: %v", err)
	}
	if err := au.AddUser("alice", "alicepass123", role); err != nil {
		t.Fatalf("add %s: %v", role, err)
	}
	rec := req(t, h, http.MethodPost, "/api/auth/login", remoteAddr,
		`{"username":"alice","password":"alicepass123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login as %s: want 200, got %d (%s)", role, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatalf("login as %s set no session cookie", role)
	return nil
}

// Destructive and machine-wide routes must be owner-only. Before this, every
// authenticated session could purge the install, flip remote access, point
// off-site backups at an arbitrary host directory, or re-link the account.
func TestOwnerOnlyRoutesRejectNonOwners(t *testing.T) {
	for _, role := range []string{auth.RoleAdmin, auth.RoleOperator} {
		t.Run(role, func(t *testing.T) {
			h, _, au := newTestAPI(t)
			cookie := operatorSession(t, h, au, role)

			cases := []struct{ method, path, body string }{
				{http.MethodPost, "/api/system/purge", `{"confirm":"DELETE"}`},
				{http.MethodPost, "/api/system/remote-access", `{"enabled":true}`},
				{http.MethodPost, "/api/system/offsite", `{"dir":"C:\\"}`},
				{http.MethodPost, "/api/system/telemetry", `{"enabled":true}`},
				{http.MethodPost, "/api/system/setup/docker", ``},
				{http.MethodPost, "/api/license", `{"key":"x"}`},
				{http.MethodDelete, "/api/license", ``},
				{http.MethodPost, "/api/account/link", `{"code":"x"}`},
				{http.MethodDelete, "/api/account/link", ``},
				{http.MethodPost, "/api/users", `{"username":"bob","password":"bobpass12345","role":"operator"}`},
				{http.MethodDelete, "/api/users/alice", ``},
			}
			for _, c := range cases {
				rec := req(t, h, c.method, c.path, remoteAddr, c.body, cookie)
				if rec.Code != http.StatusForbidden {
					t.Errorf("%s %s as %s: want 403, got %d (%s)",
						c.method, c.path, role, rec.Code, rec.Body.String())
				}
			}
		})
	}
}

// The gate is on the role, not on being remote: an operator keeps full control
// of game servers, which is what the account exists for.
func TestOperatorKeepsServerManagement(t *testing.T) {
	h, mgr, au := newTestAPI(t)
	cookie := operatorSession(t, h, au, auth.RoleOperator)

	if _, err := mgr.Create(server.CreateRequest{TemplateID: "test-mc", Name: "X", Port: 25565}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// (/api/system/network is left out: it needs a real port mapper, which the
	// test API has no business wiring up just to prove a role gate.)
	for _, path := range []string{"/api/servers", "/api/system/runtime", "/api/license", "/api/account"} {
		if rec := req(t, h, http.MethodGet, path, remoteAddr, "", cookie); rec.Code != http.StatusOK {
			t.Errorf("GET %s as operator: want 200, got %d (%s)", path, rec.Code, rec.Body.String())
		}
	}
}

// Loopback is the owner, so the local desktop app reaches everything as before.
func TestLoopbackStillReachesOwnerOnlyRoutes(t *testing.T) {
	h, _, au := newTestAPI(t)
	if err := au.SetPassword("ownerpass123"); err != nil {
		t.Fatalf("set owner password: %v", err)
	}

	// A 403 here would mean the middleware locked the owner out of their own
	// machine; any other status is the handler doing its normal job.
	rec := req(t, h, http.MethodPost, "/api/system/telemetry", "127.0.0.1:51000", `{"enabled":true}`)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("loopback owner was refused: %d (%s)", rec.Code, rec.Body.String())
	}
}
