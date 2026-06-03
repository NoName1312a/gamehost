package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// relayStatus reports the playit relay state (installed/linked/running + URLs).
func (a *API) relayStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.relay.Status())
}

// relayAction runs a relay control action: install, start, stop, open-setup,
// open-dashboard.
func (a *API) relayAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := a.relay.RunAction(ctx, chi.URLParam(r, "action")); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// setRelayAddress stores the playit address a user pasted back for a server.
func (a *API) setRelayAddress(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid request body"))
		return
	}
	if err := a.mgr.SetRelayAddress(chi.URLParam(r, "id"), req.Address); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
