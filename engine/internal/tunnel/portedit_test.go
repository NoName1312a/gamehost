package tunnel

import (
	"context"
	"os"
	"strings"
	"testing"
)

func mcWant(localPort int, ports ...PortReq) []Desired {
	if len(ports) == 0 {
		ports = []PortReq{{Role: "game", Proto: "udp"}}
	}
	local := map[string]int{}
	for i, p := range ports {
		local[p.Role] = localPort + i
	}
	return []Desired{{Slug: "mc", Ports: ports, LocalPorts: local}}
}

// Regression: editing a tunneled server's port left frpc pointed at the old
// local port. The slug was still in the wanted set, so nothing counted as
// "changed", frpc.toml was never re-rendered, and the public address silently
// stopped reaching the server — with the UI still displaying it as live.
func TestReconcileRewritesWhenLocalPortChanges(t *testing.T) {
	rec := newCP()
	srv := rec.server(t)
	dir := t.TempDir()
	fake := &fakeSup{}
	a := newAgent(NewClient(srv.URL, dir), fake, dir)

	if _, err := a.Reconcile(context.Background(), mcWant(25565)); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	restartsAfterAdd := fake.restarts

	// The user changes the server's port. Same slug, same roles — only the
	// local port moves.
	if _, err := a.Reconcile(context.Background(), mcWant(25577)); err != nil {
		t.Fatalf("reconcile after port edit: %v", err)
	}

	cfg, err := os.ReadFile(a.cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(cfg), "localPort = 25577") {
		t.Errorf("frpc.toml still points at the old port:\n%s", cfg)
	}
	if strings.Contains(string(cfg), "localPort = 25565") {
		t.Errorf("frpc.toml kept the stale port:\n%s", cfg)
	}
	if fake.restarts <= restartsAfterAdd {
		t.Errorf("frpc was not restarted for the new config (restarts=%d)", fake.restarts)
	}
	// A local port change needs no new public port.
	if rec.allocates["mc"] != 1 {
		t.Errorf("re-allocated for a local port change: %d", rec.allocates["mc"])
	}
}

// The rewrite is gated on the rendered config, so an unchanged want must still
// not churn frpc — restarting drops every player mid-session.
func TestReconcileDoesNotRestartWhenNothingChanges(t *testing.T) {
	rec := newCP()
	srv := rec.server(t)
	dir := t.TempDir()
	fake := &fakeSup{}
	a := newAgent(NewClient(srv.URL, dir), fake, dir)

	if _, err := a.Reconcile(context.Background(), mcWant(25565)); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := a.Reconcile(context.Background(), mcWant(25565)); err != nil {
			t.Fatalf("repeat reconcile %d: %v", i, err)
		}
	}
	if fake.restarts != 1 {
		t.Errorf("frpc restarted %d times for an unchanged want, want 1", fake.restarts)
	}
}

// Adding a port role means the public ports we hold are the wrong shape, so the
// allocation itself has to be redone — a re-render cannot conjure a second
// public port.
func TestReconcileReallocatesWhenRoleSetChanges(t *testing.T) {
	rec := newCP()
	srv := rec.server(t)
	dir := t.TempDir()
	fake := &fakeSup{}
	a := newAgent(NewClient(srv.URL, dir), fake, dir)

	if _, err := a.Reconcile(context.Background(), mcWant(25565)); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}

	withQuery := mcWant(25565,
		PortReq{Role: "game", Proto: "udp"},
		PortReq{Role: "query", Proto: "tcp"},
	)
	got, err := a.Reconcile(context.Background(), withQuery)
	if err != nil {
		t.Fatalf("reconcile after adding a role: %v", err)
	}

	if rec.releases["mc"] != 1 {
		t.Errorf("old allocation was not released: %d", rec.releases["mc"])
	}
	if rec.allocates["mc"] != 2 {
		t.Errorf("allocate count = %d, want 2 (initial + re-allocate)", rec.allocates["mc"])
	}
	if n := len(got["mc"].Proxies); n != 2 {
		t.Errorf("allocation has %d proxies, want 2", n)
	}
}

// Changing only the protocol of a role is also a real allocation change: a UDP
// public port cannot serve a TCP proxy.
func TestReconcileReallocatesWhenProtocolChanges(t *testing.T) {
	rec := newCP()
	srv := rec.server(t)
	dir := t.TempDir()
	fake := &fakeSup{}
	a := newAgent(NewClient(srv.URL, dir), fake, dir)

	if _, err := a.Reconcile(context.Background(), mcWant(25565, PortReq{Role: "game", Proto: "udp"})); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	if _, err := a.Reconcile(context.Background(), mcWant(25565, PortReq{Role: "game", Proto: "tcp"})); err != nil {
		t.Fatalf("reconcile after protocol change: %v", err)
	}

	if rec.allocates["mc"] != 2 {
		t.Errorf("allocate count = %d, want 2 — protocol change needs a new allocation", rec.allocates["mc"])
	}
}
