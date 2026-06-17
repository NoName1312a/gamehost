package tunnel

import (
	"context"
	"log/slog"
	"path/filepath"
	"sort"
	"sync"
)

// supervisor is the frpc process manager the Agent drives; *sidecar is the real
// implementation, and tests inject a fake.
type supervisor interface {
	restart(cfgPath string) error
	stop()
	isRunning() bool
}

// Desired is one server the engine wants tunneled: a stable slug, the public
// ports to request, and the local host port each role is bound to.
type Desired struct {
	Slug       string
	Ports      []PortReq
	LocalPorts map[string]int // role -> host port on 127.0.0.1
}

// Status is the tunnel state surfaced to the UI/API.
type Status struct {
	Configured bool   `json:"configured"`
	Running    bool   `json:"running"`
	Active     int    `json:"active"`
	Message    string `json:"message"`
}

// Agent reconciles the set of tunnel-enabled servers against the control-plane
// and a single supervised frpc process. Safe for concurrent use.
type Agent struct {
	client  *Client
	sup     supervisor
	dataDir string
	cfgPath string

	mu      sync.Mutex
	current map[string]Allocation // slug -> active allocation
}

// New returns an Agent that talks to the control-plane at baseURL and stores its
// token + frpc config under dataDir.
func New(dataDir, baseURL string) *Agent {
	return newAgent(NewClient(baseURL, dataDir), &sidecar{bin: locate()}, dataDir)
}

func newAgent(c *Client, sup supervisor, dataDir string) *Agent {
	return &Agent{
		client:  c,
		sup:     sup,
		dataDir: dataDir,
		cfgPath: filepath.Join(dataDir, "frpc.toml"),
		current: map[string]Allocation{},
	}
}

// Reconcile drives the live tunnel set toward want: it releases slugs no longer
// wanted, allocates newly wanted ones, and—only if the set changed—rewrites
// frpc.toml from all current allocations and restarts frpc. It is best-effort:
// a single server's allocate/release failure is logged, not returned, so one
// bad server can't take down the others. Returns the current allocations.
func (a *Agent) Reconcile(ctx context.Context, want []Desired) (map[string]Allocation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	wantSet := make(map[string]Desired, len(want))
	for _, d := range want {
		wantSet[d.Slug] = d
	}

	changed := false

	// Release slugs no longer wanted.
	for slug := range a.current {
		if _, ok := wantSet[slug]; ok {
			continue
		}
		if err := a.client.Release(ctx, slug); err != nil {
			slog.Warn("tunnel: release failed", "slug", slug, "err", err)
		}
		delete(a.current, slug)
		changed = true
	}

	// Allocate newly wanted slugs (MVP: existing slugs are left as-is).
	for slug, d := range wantSet {
		if _, ok := a.current[slug]; ok {
			continue
		}
		alloc, err := a.client.Allocate(ctx, slug, d.Ports)
		if err != nil {
			slog.Warn("tunnel: allocate failed", "slug", slug, "err", err)
			continue
		}
		a.current[slug] = alloc
		changed = true
	}

	if changed {
		if err := a.rewrite(wantSet); err != nil {
			slog.Warn("tunnel: frpc reconfigure failed", "err", err)
			return a.snapshot(), err
		}
	}
	return a.snapshot(), nil
}

// rewrite renders frpc.toml from all current allocations and (re)starts frpc, or
// stops it when nothing is tunneled. Caller holds a.mu.
func (a *Agent) rewrite(wantSet map[string]Desired) error {
	slugs := make([]string, 0, len(a.current))
	for s := range a.current {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs) // deterministic config order

	var proxies []localProxy
	var frpsAddr, frpsToken string
	for _, slug := range slugs {
		alloc := a.current[slug]
		frpsAddr, frpsToken = alloc.FrpsAddr, alloc.FrpsToken
		d := wantSet[slug]
		for _, p := range alloc.Proxies {
			proxies = append(proxies, localProxy{
				Name:       p.Name,
				Proto:      p.Proto,
				LocalPort:  d.LocalPorts[p.Role],
				RemotePort: p.RemotePort,
				Secret:     alloc.Secret,
			})
		}
	}

	if len(proxies) == 0 {
		a.sup.stop()
		return nil
	}
	if err := writeConfig(a.cfgPath, frpsAddr, frpsToken, proxies); err != nil {
		return err
	}
	return a.sup.restart(a.cfgPath)
}

func (a *Agent) snapshot() map[string]Allocation {
	out := make(map[string]Allocation, len(a.current))
	for k, v := range a.current {
		out[k] = v
	}
	return out
}

// Stop tears down the frpc sidecar — e.g. on engine shutdown — so the tunnel
// doesn't outlive the app. Control-plane allocations are left as-is; frps drops
// each proxy the moment frpc disconnects, and they're reclaimed on the next
// reconcile.
func (a *Agent) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sup.stop()
}

// Addresses returns the public proxies for slug, or nil if not tunneled.
func (a *Agent) Addresses(slug string) []AllocProxy {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.current[slug].Proxies
}

// Status reports the tunnel feature state for the UI/API.
func (a *Agent) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := Status{
		Configured: a.client.baseURL != "",
		Running:    a.sup.isRunning(),
		Active:     len(a.current),
	}
	switch {
	case !st.Configured:
		st.Message = "Tunnel is not configured."
	case st.Active == 0:
		st.Message = "Enable \"Share with friends\" on a server to get a public address."
	default:
		st.Message = "Tunnel is active. Share each server's address with your friends."
	}
	return st
}
