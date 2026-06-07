package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOriginAllowed verifies the WebSocket console only accepts handshakes from
// allow-listed browser origins, while non-browser clients (no Origin header)
// still connect. This closes the WS-CSRF vector from a malicious web page.
func TestOriginAllowed(t *testing.T) {
	allowed := []string{"http://localhost:5173", "tauri://localhost"}
	mk := func(origin string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/servers/x/console", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}

	cases := []struct {
		origin string
		want   bool
	}{
		{"", true},                          // non-browser client (desktop shell, curl)
		{"http://localhost:5173", true},     // dev UI
		{"tauri://localhost", true},         // desktop webview
		{"https://evil.example", false},     // malicious cross-site page
		{"http://localhost:5173.evil", false}, // lookalike must not match
	}
	for _, c := range cases {
		if got := originAllowed(mk(c.origin), allowed); got != c.want {
			t.Errorf("originAllowed(origin=%q) = %v, want %v", c.origin, got, c.want)
		}
	}
}
