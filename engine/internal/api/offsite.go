package api

import "net/http"

// offsiteStatus reports the configured off-site backup folder and whether Pro
// (required for the copy to actually run) is active.
func (a *API) offsiteStatus(w http.ResponseWriter, r *http.Request) {
	pro := false
	if a.license != nil {
		pro = a.license.IsPro()
	}
	writeJSON(w, http.StatusOK, map[string]any{"dir": a.mgr.OffsiteDir(), "pro": pro})
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
