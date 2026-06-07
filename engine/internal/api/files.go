package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// File-manager endpoints let the UI browse and edit a server's data volume
// (configs, mods, worlds). All operations go through the docker helper, so they
// work whether or not the server is running.

const (
	// maxFileWriteBytes caps the body of a file-write request so a huge payload
	// can't exhaust engine memory. Generous for text/config edits (reads are
	// truncated at ~1 MiB), while bounding a malicious upload.
	maxFileWriteBytes = 16 << 20 // 16 MiB
	// maxControlBody caps small JSON control bodies (create/update/mkdir).
	maxControlBody = 1 << 20 // 1 MiB
)

// decodeJSON caps the request body and decodes it as JSON, writing a 413/400
// response and returning false if the body is too large or malformed.
func decodeJSON(w http.ResponseWriter, r *http.Request, max int64, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, max)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeJSON(w, http.StatusRequestEntityTooLarge, errMsg("request body too large"))
			return false
		}
		writeJSON(w, http.StatusBadRequest, errMsg("invalid request body"))
		return false
	}
	return true
}

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
	if !decodeJSON(w, r, maxFileWriteBytes, &body) {
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
	if !decodeJSON(w, r, maxControlBody, &body) {
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
