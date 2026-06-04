package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/leop1/gamehost/engine/internal/server"
)

func (a *API) listServers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, a.mgr.List(ctx))
}

func (a *API) createServer(w http.ResponseWriter, r *http.Request) {
	var req server.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid request body"))
		return
	}
	s, err := a.mgr.Create(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (a *API) updateServer(w http.ResponseWriter, r *http.Request) {
	var req server.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid request body"))
		return
	}
	// Generous timeout: applying changes may stop, recreate, and restart the
	// container (and a recreate can re-pull the image).
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	s, err := a.mgr.Update(ctx, chi.URLParam(r, "id"), req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *API) startServer(w http.ResponseWriter, r *http.Request) {
	// Generous timeout: the first start pulls the image, which can be slow.
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	if err := a.mgr.Start(ctx, chi.URLParam(r, "id")); err != nil {
		writeJSON(w, http.StatusInternalServerError, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func (a *API) stopServer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if err := a.mgr.Stop(ctx, chi.URLParam(r, "id")); err != nil {
		writeJSON(w, http.StatusInternalServerError, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (a *API) deleteServer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if err := a.mgr.Delete(ctx, chi.URLParam(r, "id")); err != nil {
		writeJSON(w, http.StatusInternalServerError, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func errMsg(s string) map[string]string { return map[string]string{"error": s} }
