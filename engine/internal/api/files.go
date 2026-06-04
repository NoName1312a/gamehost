package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// File-manager endpoints let the UI browse and edit a server's data volume
// (configs, mods, worlds). All operations go through the docker helper, so they
// work whether or not the server is running.

func (a *API) serverVolume(w http.ResponseWriter, r *http.Request) (string, bool) {
	s, ok := a.mgr.Get(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, errMsg("server not found"))
		return "", false
	}
	return s.VolumeName(), true
}

func (a *API) listFiles(w http.ResponseWriter, r *http.Request) {
	vol, ok := a.serverVolume(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	rel := r.URL.Query().Get("path")
	entries, err := a.rt.ListFiles(ctx, vol, rel)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": rel, "entries": entries})
}

func (a *API) readFile(w http.ResponseWriter, r *http.Request) {
	vol, ok := a.serverVolume(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	rel := r.URL.Query().Get("path")
	content, truncated, err := a.rt.ReadFile(ctx, vol, rel)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": rel, "content": content, "truncated": truncated})
}

func (a *API) writeFile(w http.ResponseWriter, r *http.Request) {
	vol, ok := a.serverVolume(w, r)
	if !ok {
		return
	}
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid request body"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := a.rt.WriteFile(ctx, vol, body.Path, []byte(body.Content)); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (a *API) makeDir(w http.ResponseWriter, r *http.Request) {
	vol, ok := a.serverVolume(w, r)
	if !ok {
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid request body"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := a.rt.MakeDir(ctx, vol, body.Path); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "created"})
}

func (a *API) deleteFile(w http.ResponseWriter, r *http.Request) {
	vol, ok := a.serverVolume(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := a.rt.DeleteFile(ctx, vol, r.URL.Query().Get("path")); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
