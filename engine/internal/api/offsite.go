package api

import "net/http"

// offsiteStatus reports the configured off-site backup folder.
func (a *API) offsiteStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"dir": a.mgr.OffsiteDir()})
}

// setOffsite sets (or clears) the off-site backup folder.
func (a *API) setOffsite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Dir string `json:"dir"`
	}
	if !decodeJSON(w, r, maxControlBody, &body) {
		return
	}
	if err := a.mgr.SetOffsiteDir(body.Dir); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	a.offsiteStatus(w, r)
}
