package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// serverStats returns a one-shot CPU/memory sample for a running server. The UI
// polls this and keeps a rolling client-side history for the graphs.
func (a *API) serverStats(w http.ResponseWriter, r *http.Request) {
	s, ok := a.mgr.Get(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, errMsg("server not found"))
		return
	}
	// `docker stats --no-stream` samples for ~1-2s.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	st, err := a.rt.Stats(ctx, s.ContainerName())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("stats unavailable (is the server running?)"))
		return
	}
	writeJSON(w, http.StatusOK, st)
}
