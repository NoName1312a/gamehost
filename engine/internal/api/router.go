// Package api exposes the engine's HTTP surface: a small REST API today, and
// WebSocket console streams in later milestones. The browser UI (and, later,
// the Tauri shell) is the only client.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/leop1/gamehost/engine/internal/config"
	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/templates"
)

// Version is the engine API version, surfaced at /api/health.
const Version = "0.0.1-m0"

// API bundles the dependencies handlers need.
type API struct {
	cfg config.Config
	rt  *docker.Runtime
	reg *templates.Registry
}

// NewRouter wires up the HTTP routes and middleware.
func NewRouter(cfg config.Config, rt *docker.Runtime, reg *templates.Registry) http.Handler {
	a := &API{cfg: cfg, rt: rt, reg: reg}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", a.health)
		r.Get("/system/runtime", a.runtime)
		r.Get("/templates", a.listTemplates)
		r.Get("/templates/{id}", a.getTemplate)
	})

	return r
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "gamehost-engine",
		"version": Version,
	})
}

func (a *API) runtime(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.rt.Probe(r.Context()))
}

func (a *API) listTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.reg.List())
}

func (a *API) getTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, ok := a.reg.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
