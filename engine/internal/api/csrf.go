package api

import "net/http"

// csrfHeader is a non-CORS-safelisted request header the UI sends on every
// state-changing call. Requiring it is what defends the loopback engine against
// drive-by CSRF.
const csrfHeader = "X-GameNest"

// csrfGuard blocks cross-site request forgery against the local engine.
//
// The engine trusts loopback connections as the owner (no session). A web page
// the user visits while the engine runs shares that loopback path, so without a
// guard it could fire a CORS "simple" cross-origin POST (e.g. text/plain, which
// the browser sends with no preflight) at 127.0.0.1 and have it run as the owner
// — including POST /api/system/purge, which wipes every server and volume. CORS
// keeps the attacker from reading the response, but the state change still lands.
//
// Requiring a non-safelisted header on every mutating request closes this: a
// cross-origin attempt to set the header forces a CORS preflight that fails for
// any non-allow-listed origin, and a request that omits the header is rejected
// here. The legitimate UI (same-origin or allow-listed) sends it trivially.
// Reads (GET/HEAD) are left alone — CORS already prevents an attacker reading
// their responses, so they need no header and stay preflight-free.
func (a *API) csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if r.Header.Get(csrfHeader) == "" {
				writeJSON(w, http.StatusForbidden, errMsg("missing required request header"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
