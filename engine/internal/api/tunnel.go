package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/leop1/gamehost/engine/internal/server"
	"github.com/leop1/gamehost/engine/internal/tunnel"
)

// tunnelStatus reports the built-in tunnel feature state (configured/running/
// active + a UI hint). When no tunnel is wired (GAMEHOST_TUNNEL_URL unset) it
// reports configured=false so the UI can hide or disable the feature.
func (a *API) tunnelStatus(w http.ResponseWriter, r *http.Request) {
	if a.tunnel == nil {
		writeJSON(w, http.StatusOK, tunnel.Status{Message: "Tunnel is not configured."})
		return
	}
	writeJSON(w, http.StatusOK, a.tunnel.Status())
}

// setUseTunnel turns the built-in tunnel on or off for a server (PUT body
// {"on": bool}); the public address then appears/disappears in the server view.
func (a *API) setUseTunnel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		On bool `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg("invalid request body"))
		return
	}
	if err := a.mgr.SetUseTunnel(chi.URLParam(r, "id"), req.On); err != nil {
		writeJSON(w, http.StatusBadRequest, errMsg(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// tunnelReconciler is the subset of *tunnel.Agent the manager adapter drives.
// An interface (rather than the concrete agent) keeps the adapter's type
// conversion unit-testable with a fake.
type tunnelReconciler interface {
	Reconcile(ctx context.Context, want []tunnel.Desired) (map[string]tunnel.Allocation, error)
}

// tunnelAdapter bridges the manager's dependency-decoupled tunnel types to the
// tunnel package's own types, so a tunnel.Agent satisfies server.Tunnel without
// the manager importing the tunnel package.
type tunnelAdapter struct{ ag tunnelReconciler }

// AdaptTunnel wraps a tunnel.Agent so the server manager can drive it through
// Manager.SetTunnel.
func AdaptTunnel(ag *tunnel.Agent) server.Tunnel { return tunnelAdapter{ag: ag} }

func (t tunnelAdapter) Reconcile(ctx context.Context, want []server.TunnelWant) (map[string]server.TunnelAddrs, error) {
	desired := make([]tunnel.Desired, 0, len(want))
	for _, w := range want {
		d := tunnel.Desired{Slug: w.Slug, LocalPorts: w.LocalPorts}
		for _, p := range w.Ports {
			d.Ports = append(d.Ports, tunnel.PortReq{Role: p.Role, Proto: p.Proto})
		}
		desired = append(desired, d)
	}
	allocs, err := t.ag.Reconcile(ctx, desired)
	out := make(map[string]server.TunnelAddrs, len(allocs))
	for slug, a := range allocs {
		var ta server.TunnelAddrs
		for _, p := range a.Proxies {
			ta.Endpoints = append(ta.Endpoints, server.TunnelEndpoint{
				Role:    p.Role,
				Proto:   p.Proto,
				Address: p.Address,
			})
		}
		out[slug] = ta
	}
	return out, err
}
