package api

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/leop1/gamehost/engine/internal/auth"
)

// Authentication model: the engine trusts loopback connections (the local
// desktop user) and requires a valid session for everything else. With the
// engine bound to loopback (desktop default) requireAuth never rejects; it only
// bites once the engine is exposed on a network (remote-access mode).
//
// Loopback trust additionally requires a loopback Host header (see
// loopbackTrusted / hostGuard). Without it, a DNS-rebinding page — which rebinds
// its own domain to 127.0.0.1 so its requests reach the engine over a loopback
// socket while still carrying its attacker Host — would inherit owner trust.

const sessionCookie = "gh_session"
const sessionTTL = 7 * 24 * time.Hour

// isLoopback reports whether an http.Request RemoteAddr is a loopback address.
func isLoopback(remoteAddr string) bool {
	ip := net.ParseIP(clientIP(remoteAddr))
	return ip != nil && ip.IsLoopback()
}

// clientIP strips the port from an http.Request RemoteAddr.
func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// hostIsLoopback reports whether an HTTP Host header names a loopback address
// ("localhost", 127.0.0.0/8, or ::1). The desktop UI always addresses the engine
// at a loopback host; a DNS-rebinding page cannot forge one, because the browser
// derives Host from the request URL's authority (its rebound domain).
func hostIsLoopback(host string) bool {
	if host == "" {
		return false
	}
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host // no port present
	}
	h = strings.Trim(h, "[]") // unwrap a bracketed IPv6 literal
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// loopbackTrusted reports whether a request may act as the local owner without a
// session: it must arrive over a loopback socket AND address a loopback Host. The
// Host check is what defeats DNS rebinding (see hostGuard).
func (a *API) loopbackTrusted(r *http.Request) bool {
	return isLoopback(r.RemoteAddr) && hostIsLoopback(r.Host)
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

// authed reports whether the request is allowed: trusted loopback or a valid
// session.
func (a *API) authed(r *http.Request) bool {
	return a.loopbackTrusted(r) || a.auth.ValidateSession(sessionToken(r))
}

// effectiveUser returns the acting account's username and role. Loopback (the
// local desktop user) is treated as the owner.
func (a *API) effectiveUser(r *http.Request) (username, role string) {
	if a.loopbackTrusted(r) {
		return "owner", auth.RoleOwner
	}
	if u, ok := a.auth.SessionUsername(sessionToken(r)); ok {
		if role, ok := a.auth.UserRole(u); ok {
			return u, role
		}
	}
	return "", ""
}

// requireOwner reports whether the caller may manage users.
func (a *API) requireOwner(r *http.Request) bool {
	_, role := a.effectiveUser(r)
	return role == auth.RoleOwner
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
	user, role := a.effectiveUser(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": a.authed(r),
		"hasPassword":   a.auth.HasPassword(),
		"loopback":      a.loopbackTrusted(r),
		"user":          user,
		"role":          role,
	})
}

// authLogin verifies the operator password and sets a session cookie.
func (a *API) authLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r.RemoteAddr)
	if a.loginLimiter.blocked(ip, time.Now()) {
		writeJSON(w, http.StatusTooManyRequests, errMsg("too many login attempts; please wait and try again"))
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, maxControlBody, &body) {
		return
	}
	username := strings.TrimSpace(body.Username)
	ok := false
	if username == "" {
		ok = a.auth.Verify(body.Password) // owner (back-compat with password-only login)
	} else {
		ok = a.auth.VerifyUser(username, body.Password)
	}
	if !ok {
		a.loginLimiter.fail(ip, time.Now())
		writeJSON(w, http.StatusUnauthorized, errMsg("incorrect username or password"))
		return
	}
	a.loginLimiter.reset(ip)
	tok := a.auth.CreateSession(sessionTTL)
	if username != "" {
		tok = a.auth.CreateSessionFor(username, sessionTTL)
	}
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
