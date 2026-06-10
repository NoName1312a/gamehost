package api

import "net/http"

// telemetryStatus reports whether anonymous diagnostics are enabled. The UI
// renders the opt-in toggle from this.
func (a *API) telemetryStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": a.telemetry.IsEnabled(),
		"version": Version,
	})
}

// setTelemetry records the user's opt-in choice for anonymous diagnostics.
func (a *API) setTelemetry(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeJSON(w, r, maxControlBody, &body) {
		return
	}
	if err := a.telemetry.SetEnabled(body.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, errMsg("could not save preference: "+err.Error()))
		return
	}
	a.telemetryStatus(w, r)
}
