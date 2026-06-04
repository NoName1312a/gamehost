package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// setSchedule updates a server's daily restart/backup times. The current values
// are already surfaced on the server object (restartAt/backupAt), so there's no
// separate GET. Empty strings disable a schedule.
func (a *API) setSchedule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RestartAt string `json:"restartAt"`
		BackupAt  string `json:"backupAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid request body"))
		return
	}
	if err := a.mgr.SetSchedule(chi.URLParam(r, "id"), body.RestartAt, body.BackupAt); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
