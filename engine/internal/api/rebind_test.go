package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// loopbackWithHost issues a request over a loopback socket but with a caller-
// chosen Host header — the exact shape of a DNS-rebinding attack, where a page
// rebinds its domain to 127.0.0.1 (loopback socket) yet the browser still sends
// the attacker's domain as Host. The anti-CSRF header is set because, post-rebind,
// the page is same-origin and can set it freely — so it must NOT be what saves us.
func loopbackWithHost(t *testing.T, h http.Handler, method, path, host string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr = "127.0.0.1:50000"
	r.Host = host
	r.Header.Set(csrfHeader, "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// A loopback request whose Host is not a loopback name is a DNS-rebinding attempt
// and must be refused — reads (which would leak state to the attacker page) and
// writes (which would run as the owner, e.g. wiping every server) alike.
func TestDNSRebindingIsRejected(t *testing.T) {
	h, _, _ := newTestAPI(t)
	cases := []struct {
		name, method, path string
	}{
		{"read servers", http.MethodGet, "/api/servers"},
		{"read a file", http.MethodGet, "/api/servers/x/files/read?path=/data"},
		{"purge everything", http.MethodPost, "/api/system/purge"},
		{"create a server", http.MethodPost, "/api/servers"},
		{"public auth status", http.MethodGet, "/api/auth/status"},
	}
	for _, c := range cases {
		rec := loopbackWithHost(t, h, c.method, c.path, "evil.example.com:8723")
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s via rebinding Host: want 403, got %d (%s)", c.name, rec.Code, rec.Body.String())
		}
	}
}

// The legitimate desktop UI always reaches the engine at a loopback Host, which
// must keep its no-session owner trust across the loopback name variants.
func TestLoopbackHostsKeepOwnerTrust(t *testing.T) {
	h, _, _ := newTestAPI(t)
	for _, host := range []string{"127.0.0.1:8723", "localhost:8723", "[::1]:8723", "127.0.0.1"} {
		rec := loopbackWithHost(t, h, http.MethodGet, "/api/servers", host)
		if rec.Code != http.StatusOK {
			t.Errorf("loopback Host %q: want 200, got %d (%s)", host, rec.Code, rec.Body.String())
		}
	}
}

// hostIsLoopback is the core check; verify its edges directly.
func TestHostIsLoopback(t *testing.T) {
	loopback := []string{"127.0.0.1:8723", "127.0.0.1", "localhost:8723", "localhost",
		"LOCALHOST", "[::1]:8723", "::1", "127.5.6.7:9"}
	for _, h := range loopback {
		if !hostIsLoopback(h) {
			t.Errorf("hostIsLoopback(%q) = false, want true", h)
		}
	}
	notLoopback := []string{"", "evil.com", "evil.com:8723", "example.com",
		"192.168.1.10:8723", "10.0.0.5", "0.0.0.0:8723", "127.0.0.1.evil.com"}
	for _, h := range notLoopback {
		if hostIsLoopback(h) {
			t.Errorf("hostIsLoopback(%q) = true, want false", h)
		}
	}
}
