package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leop1/gamehost/engine/internal/server"
)

const remoteAddr = "203.0.113.7:40000" // non-loopback (TEST-NET-3)

// req issues a request from a given RemoteAddr with optional cookies.
func req(t *testing.T, h http.Handler, method, path, remote, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r.RemoteAddr = remote
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestNonLoopbackRequiresAuth(t *testing.T) {
	h, mgr, au := newTestAPI(t)
	if err := au.SetPassword("password123"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	if _, err := mgr.Create(server.CreateRequest{TemplateID: "test-mc", Name: "X", Port: 25565}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// No session, non-loopback -> 401.
	if rec := req(t, h, http.MethodGet, "/api/servers", remoteAddr, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated remote: want 401, got %d", rec.Code)
	}

	// Wrong password -> 401, no cookie.
	if rec := req(t, h, http.MethodPost, "/api/auth/login", remoteAddr, `{"password":"nope"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: want 401, got %d", rec.Code)
	}

	// Correct password -> 200 + session cookie.
	rec := req(t, h, http.MethodPost, "/api/auth/login", remoteAddr, `{"password":"password123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("login did not set a session cookie")
	}

	// With the session cookie, the same remote request now succeeds.
	if rec := req(t, h, http.MethodGet, "/api/servers", remoteAddr, "", cookie); rec.Code != http.StatusOK {
		t.Fatalf("authenticated remote: want 200, got %d", rec.Code)
	}
}

func TestLoopbackBypassesAuth(t *testing.T) {
	h, _, au := newTestAPI(t)
	_ = au.SetPassword("password123")
	// Loopback is trusted even with a password set and no session.
	if rec := req(t, h, http.MethodGet, "/api/servers", "127.0.0.1:50000", ""); rec.Code != http.StatusOK {
		t.Fatalf("loopback: want 200, got %d", rec.Code)
	}
}

func TestHealthIsPublic(t *testing.T) {
	h, _, _ := newTestAPI(t)
	if rec := req(t, h, http.MethodGet, "/api/health", remoteAddr, ""); rec.Code != http.StatusOK {
		t.Fatalf("health should be public: want 200, got %d", rec.Code)
	}
}

func TestLoginThrottledAfterRepeatedFailures(t *testing.T) {
	h, _, au := newTestAPI(t)
	if err := au.SetPassword("password123"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	// The limiter allows 5 failures; the 6th attempt from the same IP is 429.
	saw429 := false
	for i := 0; i < 7; i++ {
		rec := req(t, h, http.MethodPost, "/api/auth/login", remoteAddr, `{"password":"wrong"}`)
		if rec.Code == http.StatusTooManyRequests {
			saw429 = true
			break
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: want 401, got %d", i, rec.Code)
		}
	}
	if !saw429 {
		t.Error("expected a 429 after repeated failed logins")
	}
}

func TestUserManagementIsOwnerOnly(t *testing.T) {
	h, _, au := newTestAPI(t)
	if err := au.SetPassword("ownerpass1"); err != nil {
		t.Fatalf("set owner: %v", err)
	}
	// Loopback (owner) can add an operator.
	if rec := do(t, h, http.MethodPost, "/api/users", `{"username":"alice","password":"alicepw12","role":"operator"}`); rec.Code != http.StatusCreated {
		t.Fatalf("owner add user: want 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	// Alice logs in remotely.
	rec := req(t, h, http.MethodPost, "/api/auth/login", remoteAddr, `{"username":"alice","password":"alicepw12"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("alice login: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie for alice")
	}
	// Alice (operator) can reach normal endpoints but NOT user management.
	if rec := req(t, h, http.MethodGet, "/api/servers", remoteAddr, "", cookie); rec.Code != http.StatusOK {
		t.Errorf("operator should reach /servers: got %d", rec.Code)
	}
	if rec := req(t, h, http.MethodGet, "/api/users", remoteAddr, "", cookie); rec.Code != http.StatusForbidden {
		t.Errorf("operator must not manage users: want 403, got %d", rec.Code)
	}
}

func TestRemoteAccessRequiresPassword(t *testing.T) {
	h, _, _ := newTestAPI(t)
	// Enabling remote access without an operator password is rejected.
	if rec := do(t, h, http.MethodPost, "/api/system/remote-access", `{"enabled":true}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("enable without password: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}
	// Status shows it disabled with no password set.
	rec := do(t, h, http.MethodGet, "/api/system/remote-access", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) || !strings.Contains(rec.Body.String(), `"hasPassword":false`) {
		t.Fatalf("remote-access status: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAuthStatusReportsState(t *testing.T) {
	h, _, _ := newTestAPI(t)
	// Non-loopback, no password: not authenticated, no password.
	rec := req(t, h, http.MethodGet, "/api/auth/status", remoteAddr, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"authenticated":false`) || !strings.Contains(body, `"hasPassword":false`) {
		t.Errorf("unexpected status body: %s", body)
	}
}
