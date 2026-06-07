package api

import (
	"net/http"

	"github.com/leop1/gamehost/engine/internal/remote"
)

// remoteAccessStatus reports whether remote access (the HTTPS-from-anywhere
// listener) is on, where it's bound, and whether the prerequisite password is
// set. The UI uses this to render the remote-access settings card.
func (a *API) remoteAccessStatus(w http.ResponseWriter, r *http.Request) {
	s := a.remote.Status()
	out := map[string]any{
		"enabled":     s.Enabled,
		"port":        s.Port,
		"addr":        s.Addr,
		"hasPassword": a.auth.HasPassword(),
	}
	if a.net != nil {
		out["externalIP"] = a.net.ExternalIP()
	}
	writeJSON(w, http.StatusOK, out)
}

// setRemoteAccess enables or disables the remote listener. Enabling requires an
// operator password (otherwise anyone on the network could reach the panel).
func (a *API) setRemoteAccess(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
		Port    int  `json:"port"`
	}
	if !decodeJSON(w, r, maxControlBody, &body) {
		return
	}
	if body.Enabled {
		if !a.auth.HasPassword() {
			writeJSON(w, http.StatusBadRequest, errMsg("set a password before enabling remote access"))
			return
		}
		port := body.Port
		if port == 0 {
			port = remote.DefaultPort
		}
		if err := a.remote.Enable(port); err != nil {
			writeJSON(w, http.StatusInternalServerError, errMsg("could not start remote access: "+err.Error()))
			return
		}
	} else {
		_ = a.remote.Disable()
	}
	a.remoteAccessStatus(w, r)
}
