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
// ports to request, the local host port each role is bound to, and an optional
// entitlement token (opaque; forwarded verbatim to the control-plane so it can
// grant a reserved vanity slug; "" for the free anonymous path).
type Desired struct {
	Slug        string
	Ports       []PortReq
	LocalPorts  map[string]int // role -> host port on 127.0.0.1
	Entitlement string         // optional; non-empty only for paying subscribers
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
	// rendered is the frpc.toml frpc is currently running. Comparing against it
	// is what decides whether to restart, so a change the allocation set cannot
	// see — an edited local port, say — still reaches frpc.
	rendered string
}

// New returns an Agent that talks to the control-plane at baseURL and stores its
// token + frpc config under dataDir.
func New(dataDir, baseURL string) *Agent {
	return newAgent(NewClient(baseURL, dataDir), &dockerSidecar{}, dataDir)
}

func newAgent(c *Client, sup supervisor, dataDir string) *Agent {
	return &Agent{
		client:  c,
		sup:     sup,
		dataDir: dataDir,
		cfgPath: filepath.Join(dataDir, "tunnel", "frpc.toml"),
		current: map[string]Allocation{},
	}
}

// Reconcile drives the live tunnel set toward want: it releases slugs no longer
// wanted, re-allocates ones whose requested ports changed, allocates newly
// wanted ones, and rewrites frpc.toml whenever the rendered config differs from
// what frpc is running. It is best-effort: a single server's allocate/release
// failure is logged, not returned, so one bad server can't take down the others.
// Returns the current allocations.
//
// Membership alone used to gate the rewrite, which meant editing a tunneled
// server's port left frpc pointed at the old local port: the slug was still
// present, so nothing was considered changed and the public address quietly
// stopped working. Comparing rendered configs catches that and still avoids
// restarting frpc when nothing actually moved.
func (a *Agent) Reconcile(ctx context.Context, want []Desired) (map[string]Allocation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	wantSet := make(map[string]Desired, len(want))
	for _, d := range want {
		wantSet[d.Slug] = d
	}

	// Release slugs no longer wanted.
	for slug := range a.current {
		if _, ok := wantSet[slug]; ok {
			continue
		}
		if err := a.client.Release(ctx, slug); err != nil {
			slog.Warn("tunnel: release failed", "slug", slug, "err", err)
		}
		delete(a.current, slug)
	}

	// Drop allocations that no longer describe what the server asks for — a
	// role or protocol was added, removed or changed. The public ports we hold
	// are the wrong shape, so the allocation has to be redone rather than
	// re-rendered; the loop below then treats the slug as new.
	for slug, d := range wantSet {
		alloc, ok := a.current[slug]
		if !ok || allocSatisfies(alloc, d) {
			continue
		}
		slog.Info("tunnel: requested ports changed, re-allocating", "slug", slug)
		if err := a.client.Release(ctx, slug); err != nil {
			slog.Warn("tunnel: release before re-allocate failed", "slug", slug, "err", err)
		}
		delete(a.current, slug)
	}

	// Allocate slugs we do not hold an allocation for.
	for slug, d := range wantSet {
		if _, ok := a.current[slug]; ok {
			continue
		}
		alloc, err := a.client.Allocate(ctx, slug, d.Ports, d.Entitlement)
		if err != nil {
			slog.Warn("tunnel: allocate failed", "slug", slug, "err", err)
			continue
		}
		a.current[slug] = alloc
	}

	if err := a.rewrite(wantSet); err != nil {
		slog.Warn("tunnel: frpc reconfigure failed", "err", err)
		return a.snapshot(), err
	}
	return a.snapshot(), nil
}

// allocSatisfies reports whether an existing allocation still covers what the
// server asks for: the same roles carrying the same protocols. A changed local
// port is deliberately not a mismatch — that only needs a re-render, not a new
// public port.
func allocSatisfies(alloc Allocation, d Desired) bool {
	if len(alloc.Proxies) != len(d.Ports) {
		return false
	}
	want := make(map[string]string, len(d.Ports))
	for _, p := range d.Ports {
		want[p.Role] = p.Proto
	}
	for _, p := range alloc.Proxies {
		proto, ok := want[p.Role]
		if !ok || proto != p.Proto {
			return false
		}
	}
	return true
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
		if a.rendered != "" {
			a.sup.stop()
			a.rendered = ""
		}
		return nil
	}

	// Restart frpc only when the config it is running would actually differ.
	cfg := renderConfig(frpsAddr, frpsToken, proxies)
	if cfg == a.rendered && a.sup.isRunning() {
		return nil
	}
	if err := writeConfig(a.cfgPath, frpsAddr, frpsToken, proxies); err != nil {
		return err
	}
	if err := a.sup.restart(a.cfgPath); err != nil {
		return err
	}
	a.rendered = cfg
	return nil
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
