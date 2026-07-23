package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// rawLoopback issues a request from loopback WITHOUT the anti-CSRF header — the
// shape a drive-by cross-site request against the local engine would take (a
// CORS "simple" request the browser sends with no preflight).
func rawLoopback(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr = "127.0.0.1:50000"
	r.Host = "127.0.0.1:8723" // loopback host; this suite exercises the CSRF guard, not hostGuard
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// A mutating request without the anti-CSRF header must be rejected, even from
// loopback — otherwise a web page the user visits while the engine is running
// could fire a cross-origin POST at 127.0.0.1 that runs as the owner (e.g.
// wiping every server + volume via /api/system/purge).
func TestMutatingRequestRequiresCSRFHeader(t *testing.T) {
	h, _, _ := newTestAPI(t)
	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/system/purge"},
		{http.MethodPost, "/api/servers"},
		{http.MethodDelete, "/api/servers/whatever"},
		{http.MethodPut, "/api/servers/whatever/schedule"},
	}
	for _, c := range cases {
		rec := rawLoopback(t, h, c.method, c.path)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s without CSRF header: want 403, got %d (%s)", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
}

// Reads (GET) are not state-changing and must keep working without the header,
// so the desktop flow and polling stay simple (no forced preflight).
func TestSafeRequestAllowedWithoutCSRFHeader(t *testing.T) {
	h, _, _ := newTestAPI(t)
	if rec := rawLoopback(t, h, http.MethodGet, "/api/health"); rec.Code != http.StatusOK {
		t.Fatalf("GET /api/health without header: want 200, got %d", rec.Code)
	}
}
