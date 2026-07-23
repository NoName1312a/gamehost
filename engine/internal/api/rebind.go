package api

import "net/http"

// hostGuard blocks DNS-rebinding attacks against the loopback engine.
//
// The engine grants owner trust to any loopback connection (see auth.go). A
// malicious web page the user visits can rebind its own domain to 127.0.0.1: its
// requests then reach the engine over a loopback socket AND are same-origin, so
// the page freely sets the anti-CSRF header (csrfGuard) — yet the browser still
// sends the attacker's domain as the Host header, because Host is derived from
// the request URL's authority and cannot be overridden by script.
//
// Requiring every loopback-originated request to also carry a loopback Host closes
// this: a rebound request (loopback socket, attacker Host) is rejected here before
// it can reach a handler or inherit owner trust, defeating drive-by reads (e.g.
// exfiltrating server files) and writes (e.g. /api/system/purge wiping all data).
//
// Requests on the remote-access listener have a non-loopback RemoteAddr and pass
// through untouched — they are gated by a session, not by loopback trust.
func (a *API) hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isLoopback(r.RemoteAddr) && !hostIsLoopback(r.Host) {
			writeJSON(w, http.StatusForbidden, errMsg("invalid host header"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
