// Package api exposes the engine's HTTP surface: a REST API plus a WebSocket
// console stream. The browser UI (and, later, the Tauri shell) is the only
// client.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/leop1/gamehost/engine/internal/account"
	"github.com/leop1/gamehost/engine/internal/audit"
	"github.com/leop1/gamehost/engine/internal/auth"
	"github.com/leop1/gamehost/engine/internal/config"
	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/license"
	"github.com/leop1/gamehost/engine/internal/network"
	"github.com/leop1/gamehost/engine/internal/relay"
	"github.com/leop1/gamehost/engine/internal/remote"
	"github.com/leop1/gamehost/engine/internal/server"
	"github.com/leop1/gamehost/engine/internal/telemetry"
	"github.com/leop1/gamehost/engine/internal/templates"
	"github.com/leop1/gamehost/engine/internal/tunnel"
)

// Version is the engine API version, surfaced at /api/health.
const Version = "0.1.0-m1"

// Deps bundles everything the router needs. Grouping them keeps the signature
// stable as new subsystems (auth, remote, audit, license, …) are added.
type Deps struct {
	Cfg       config.Config
	RT        *docker.Runtime
	Reg       *templates.Registry
	Mgr       *server.Manager
	Net       *network.Mapper
	Relay     *relay.Agent
	Tunnel    *tunnel.Agent
	Auth      *auth.Store
	Remote    *remote.Controller
	Audit     *audit.Logger
	License   *license.Store
	Telemetry *telemetry.Store
	// Account is the GameNest platform account store. Nil (GAMENEST_PLATFORM_URL
	// unset) leaves account routes dormant: GET /api/account reports
	// configured=false, link/unlink return 503.
	Account *account.Store
}

// API bundles the dependencies handlers need.
type API struct {
	cfg          config.Config
	rt           *docker.Runtime
	reg          *templates.Registry
	mgr          *server.Manager
	net          *network.Mapper
	relay        *relay.Agent
	tunnel       *tunnel.Agent
	auth         *auth.Store
	remote       *remote.Controller
	audit        *audit.Logger
	license      *license.Store
	telemetry    *telemetry.Store
	account      *account.Store
	loginLimiter *loginLimiter
}

// NewRouter wires up the HTTP routes and middleware. It also hands the assembled
// handler to the remote controller so the remote listener serves the same API.
func NewRouter(d Deps) http.Handler {
	a := &API{cfg: d.Cfg, rt: d.RT, reg: d.Reg, mgr: d.Mgr, net: d.Net, relay: d.Relay, tunnel: d.Tunnel,
		auth: d.Auth, remote: d.Remote, audit: d.Audit, license: d.License, telemetry: d.Telemetry,
		account: d.Account, loginLimiter: newLoginLimiter(5, 5*time.Minute)}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.Cfg.AllowOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization", csrfHeader},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	// Anti-CSRF: mutating requests must carry the csrfHeader (see csrfGuard).
	r.Use(a.csrfGuard)

	r.Route("/api", func(r chi.Router) {
		// Record mutating actions (covers login attempts too).
		r.Use(a.auditMiddleware)

		// Public: liveness + the handshake needed to authenticate.
		r.Get("/health", a.health)
		r.Get("/auth/status", a.authStatus)
		r.Post("/auth/login", a.authLogin)

		// Protected: loopback is trusted; non-loopback needs a session.
		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)

			r.Post("/auth/logout", a.authLogout)
			r.Post("/auth/password", a.authSetPassword)

			r.Get("/license", a.licenseStatus)
			r.Post("/license", a.setLicense)
			r.Delete("/license", a.clearLicense)

			r.Get("/account", a.accountStatus)
			r.Post("/account/link", a.linkAccount)
			r.Delete("/account/link", a.unlinkAccount)

			r.Get("/users", a.listUsers)
			r.Post("/users", a.addUser)
			r.Delete("/users/{username}", a.deleteUser)

			r.Get("/system/runtime", a.runtime)
			r.Get("/system/setup", a.setupReport)
			r.Post("/system/setup/{step}", a.runSetupStep)
			r.Get("/system/network", a.networkStatus)
			r.Get("/system/remote-access", a.remoteAccessStatus)
			r.Post("/system/remote-access", a.setRemoteAccess)
			r.Get("/system/offsite", a.offsiteStatus)
			r.Post("/system/offsite", a.setOffsite)
			r.Get("/system/telemetry", a.telemetryStatus)
			r.Post("/system/telemetry", a.setTelemetry)
			r.Post("/system/purge", a.purgeData)
			r.Get("/system/relay", a.relayStatus)
			r.Post("/system/relay/link", a.relayLink)
			r.Post("/system/relay/{action}", a.relayAction)
			r.Get("/system/tunnel", a.tunnelStatus)

			r.Get("/templates", a.listTemplates)
			r.Get("/templates/{id}", a.getTemplate)

			r.Get("/servers", a.listServers)
			r.Post("/servers", a.createServer)
			r.Patch("/servers/{id}", a.updateServer)
			r.Put("/servers/{id}/relay-address", a.setRelayAddress)
			r.Put("/servers/{id}/use-tunnel", a.setUseTunnel)
			r.Put("/servers/{id}/vanity", a.setVanitySlug)
			r.Get("/servers/{id}/connectivity", a.connectivity)
			r.Post("/servers/{id}/connectivity/test", a.connectivityTest)
			r.Get("/servers/{id}/files", a.listFiles)
			r.Get("/servers/{id}/files/read", a.readFile)
			r.Put("/servers/{id}/files", a.writeFile)
			r.Post("/servers/{id}/files/mkdir", a.makeDir)
			r.Delete("/servers/{id}/files", a.deleteFile)
			r.Get("/servers/{id}/backups", a.listBackups)
			r.Post("/servers/{id}/backups", a.createBackup)
			r.Post("/servers/{id}/backups/restore", a.restoreBackup)
			r.Delete("/servers/{id}/backups", a.deleteBackup)
			r.Put("/servers/{id}/schedule", a.setSchedule)
			r.Put("/servers/{id}/mods", a.setMods)
			r.Get("/servers/{id}/stats", a.serverStats)
			r.Get("/servers/{id}/console", a.console) // WebSocket
			r.Post("/servers/{id}/start", a.startServer)
			r.Post("/servers/{id}/stop", a.stopServer)
			r.Delete("/servers/{id}", a.deleteServer)
		})
	})

	if d.Remote != nil {
		d.Remote.SetHandler(r)
	}
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
