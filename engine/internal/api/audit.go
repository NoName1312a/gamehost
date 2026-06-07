package api

import "net/http"

// statusRecorder captures the response status so the audit middleware can log it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// auditMiddleware records every mutating request (non-GET) to the audit log with
// the caller's origin (local = loopback, remote = networked) and result status.
// Reads (GET/HEAD/OPTIONS) are skipped to keep the trail to state changes, and
// to avoid wrapping the WebSocket console upgrade's ResponseWriter.
func (a *API) auditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		actor := "remote"
		if isLoopback(r.RemoteAddr) {
			actor = "local"
		}
		a.audit.Record(actor, r.Method, r.URL.Path, rec.status)
	})
}
