package api

import "net/http"

// licenseStatus reports the current tier (free/pro) so the UI can show Pro
// state and gate upsells.
func (a *API) licenseStatus(w http.ResponseWriter, r *http.Request) {
	tier, email, pro := a.license.Info()
	writeJSON(w, http.StatusOK, map[string]any{"tier": tier, "email": email, "pro": pro})
}

// setLicense validates and stores a license key.
func (a *API) setLicense(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if !decodeJSON(w, r, maxControlBody, &body) {
		return
	}
	if err := a.license.Set(body.Key); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("that license key isn't valid"))
		return
	}
	a.licenseStatus(w, r)
}

// clearLicense removes the stored license (revert to free).
func (a *API) clearLicense(w http.ResponseWriter, r *http.Request) {
	_ = a.license.Clear()
	a.licenseStatus(w, r)
}
