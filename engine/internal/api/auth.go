package api

import (
	"net"
	"net/http"
	"time"
)

// Authentication model: the engine trusts loopback connections (the local
// desktop user) and requires a valid session for everything else. With the
// engine bound to loopback (desktop default) requireAuth never rejects; it only
// bites once the engine is exposed on a network (remote-access mode).

const sessionCookie = "gh_session"
const sessionTTL = 7 * 24 * time.Hour

// isLoopback reports whether an http.Request RemoteAddr is a loopback address.
func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sessionToken returns the caller's session token from the cookie or an
// Authorization: Bearer header, or "".
func sessionToken(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		return c.Value
	}
	const p = "Bearer "
	if h := r.Header.Get("Authorization"); len(h) > len(p) && h[:len(p)] == p {
		return h[len(p):]
	}
	return ""
}

// authed reports whether the request is allowed: loopback or a valid session.
func (a *API) authed(r *http.Request) bool {
	return isLoopback(r.RemoteAddr) || a.auth.ValidateSession(sessionToken(r))
}

// requireAuth is middleware that gates protected routes.
func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.authed(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, errMsg("authentication required"))
	})
}

// authStatus reports whether the caller is authenticated and whether a password
// has been set. The UI uses it to decide whether to show a login screen.
func (a *API) authStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"authenticated": a.authed(r),
		"hasPassword":   a.auth.HasPassword(),
		"loopback":      isLoopback(r.RemoteAddr),
	})
}

// authLogin verifies the operator password and sets a session cookie.
func (a *API) authLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, maxControlBody, &body) {
		return
	}
	if !a.auth.Verify(body.Password) {
		writeJSON(w, http.StatusUnauthorized, errMsg("incorrect password"))
		return
	}
	tok := a.auth.CreateSession(sessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "token": tok})
}

// authLogout invalidates the caller's session and clears the cookie.
func (a *API) authLogout(w http.ResponseWriter, r *http.Request) {
	a.auth.DeleteSession(sessionToken(r))
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// authSetPassword sets or changes the operator password. Changing an existing
// password requires the current one; the first set is open (the requireAuth
// gate already restricts who can reach this — only loopback or a session).
func (a *API) authSetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSON(w, r, maxControlBody, &body) {
		return
	}
	if len(body.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, errMsg("password must be at least 8 characters"))
		return
	}
	if a.auth.HasPassword() && !a.auth.Verify(body.CurrentPassword) {
		writeJSON(w, http.StatusUnauthorized, errMsg("current password is incorrect"))
		return
	}
	if err := a.auth.SetPassword(body.NewPassword); err != nil {
		writeJSON(w, http.StatusInternalServerError, errMsg("could not save password"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
