package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leop1/gamehost/engine/internal/server"
	"github.com/leop1/gamehost/engine/internal/tunnel"
)

// fakeReconciler captures the desired set and echoes a public address per slug,
// standing in for *tunnel.Agent so the adapter's type conversion is testable
// without the network or a real frpc.
type fakeReconciler struct {
	lastDesired []tunnel.Desired
}

func (f *fakeReconciler) Reconcile(_ context.Context, want []tunnel.Desired) (map[string]tunnel.Allocation, error) {
	f.lastDesired = want
	out := map[string]tunnel.Allocation{}
	for _, d := range want {
		a := tunnel.Allocation{Slug: d.Slug, Secret: "sec"}
		for _, p := range d.Ports {
			a.Proxies = append(a.Proxies, tunnel.AllocProxy{
				Name: "gn-" + d.Slug + "-" + p.Role, Role: p.Role, Proto: p.Proto,
				RemotePort: 39999, Address: d.Slug + ".gn.coderaum.com:39999",
			})
		}
		out[d.Slug] = a
	}
	return out, nil
}

func TestAdaptTunnelConvertsBothWays(t *testing.T) {
	fr := &fakeReconciler{}
	adapter := tunnelAdapter{ag: fr}

	out, err := adapter.Reconcile(context.Background(), []server.TunnelWant{{
		Slug:       "mc",
		Ports:      []server.TunnelPort{{Role: "tcp25565", Proto: "tcp"}},
		LocalPorts: map[string]int{"tcp25565": 25565},
	}})
	if err != nil {
		t.Fatalf("adapter reconcile: %v", err)
	}

	// server.TunnelWant -> tunnel.Desired
	if len(fr.lastDesired) != 1 || fr.lastDesired[0].Slug != "mc" {
		t.Fatalf("want not converted to Desired: %+v", fr.lastDesired)
	}
	d := fr.lastDesired[0]
	if len(d.Ports) != 1 || d.Ports[0].Role != "tcp25565" || d.Ports[0].Proto != "tcp" {
		t.Fatalf("ports not converted: %+v", d.Ports)
	}
	if d.LocalPorts["tcp25565"] != 25565 {
		t.Fatalf("local ports not converted: %+v", d.LocalPorts)
	}

	// tunnel.Allocation -> server.TunnelAddrs
	addrs, ok := out["mc"]
	if !ok || len(addrs.Endpoints) != 1 {
		t.Fatalf("allocation not converted to addrs: %+v", out)
	}
	e := addrs.Endpoints[0]
	if e.Role != "tcp25565" || e.Proto != "tcp" || e.Address != "mc.gn.coderaum.com:39999" {
		t.Fatalf("endpoint wrong: %+v", e)
	}
}

func TestTunnelStatusNotConfigured(t *testing.T) {
	h, _, _ := newTestAPI(t) // Deps carries no Tunnel -> nil agent
	req := httptest.NewRequest(http.MethodGet, "/api/system/tunnel", nil)
	req.RemoteAddr = "127.0.0.1:50000"
	req.Host = "127.0.0.1:8723"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tunnel status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["configured"] != false {
		t.Fatalf("nil tunnel should report configured=false, got %v", got)
	}
}

func TestTunnelStatusConfigured(t *testing.T) {
	a := &API{tunnel: tunnel.New(t.TempDir(), "https://cp.example")}
	req := httptest.NewRequest(http.MethodGet, "/api/system/tunnel", nil)
	rec := httptest.NewRecorder()
	a.tunnelStatus(rec, req)
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["configured"] != true {
		t.Fatalf("configured agent should report configured=true, got %v", got)
	}
}

func TestSetUseTunnelTogglesFlag(t *testing.T) {
	h, mgr, _ := newTestAPI(t)
	s, err := mgr.Create(server.CreateRequest{TemplateID: "test-mc", Name: "X", Port: 25565})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/servers/"+s.ID+"/use-tunnel", strings.NewReader(`{"on":true}`))
	req.RemoteAddr = "127.0.0.1:50000"
	req.Host = "127.0.0.1:8723"
	req.Header.Set(csrfHeader, "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set use-tunnel: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	got, _ := mgr.Get(s.ID)
	if !got.UseTunnel {
		t.Fatal("UseTunnel should be true after enabling")
	}
	if got.TunnelSlug == "" {
		t.Fatal("enabling should assign a TunnelSlug")
	}
}
