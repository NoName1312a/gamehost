package api

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
)

// backupNameRe guards restore/delete against path traversal: backup filenames
// are engine-generated timestamps, so a strict allow-list is safe.
var backupNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+\.tar\.gz$`)

func (a *API) listBackups(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := a.mgr.Get(id); !ok {
		writeJSON(w, http.StatusNotFound, errMsg("server not found"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	list, err := a.rt.ListBackups(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": list})
}

func (a *API) createBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s, ok := a.mgr.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, errMsg("server not found"))
		return
	}
	file := time.Now().UTC().Format("2006-01-02_15-04-05") + ".tar.gz"
	// Generous: archiving a large world can take a while.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	if err := a.rt.CreateBackup(ctx, s.VolumeName(), id, file); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"file": file})
}

func (a *API) restoreBackup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		File string `json:"file"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid request body"))
		return
	}
	if !backupNameRe.MatchString(body.File) {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid backup name"))
		return
	}
	// Restore stops, extracts, and may restart the server.
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	if err := a.mgr.RestoreBackup(ctx, chi.URLParam(r, "id"), body.File); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

func (a *API) deleteBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	file := r.URL.Query().Get("file")
	if !backupNameRe.MatchString(file) {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid backup name"))
		return
	}
	if _, ok := a.mgr.Get(id); !ok {
		writeJSON(w, http.StatusNotFound, errMsg("server not found"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := a.rt.DeleteBackup(ctx, id, file); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
