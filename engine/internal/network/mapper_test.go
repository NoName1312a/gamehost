package network

import (
	"context"
	"fmt"
	"testing"
)

// fakeIGD is an in-memory igdClient for tests (no real router).
type fakeIGD struct {
	added   map[string]bool
	deleted map[string]bool
}

func newFakeIGD() *fakeIGD { return &fakeIGD{added: map[string]bool{}, deleted: map[string]bool{}} }

func (f *fakeIGD) GetExternalIPAddress() (string, error) { return "203.0.113.7", nil }
func (f *fakeIGD) AddPortMapping(_ string, ep uint16, proto string, _ uint16, _ string, _ bool, _ string, _ uint32) error {
	f.added[fmt.Sprintf("%s:%d", proto, ep)] = true
	return nil
}
func (f *fakeIGD) DeletePortMapping(_ string, ep uint16, proto string) error {
	f.deleted[fmt.Sprintf("%s:%d", proto, ep)] = true
	return nil
}

// newWithClient builds a Mapper with discovery already "done" and a fixed
// client, so tests never perform real SSDP.
func newWithClient(c igdClient, ip string) *Mapper {
	m := New()
	m.once.Do(func() {}) // consume the Once so start() never spawns discovery
	if c != nil {
		m.clients = []igdClient{c}
	}
	m.externalIP = ip
	close(m.done)
	return m
}

func TestKeyRoundTrip(t *testing.T) {
	if proto, port := parseKey(key("TCP", 25565)); proto != "TCP" || port != 25565 {
		t.Fatalf("round trip: %s %d", proto, port)
	}
}

func TestMapUnmapBookkeeping(t *testing.T) {
	f := newFakeIGD()
	m := newWithClient(f, "203.0.113.7")
	ctx := context.Background()

	if err := m.Map(ctx, 25565, "tcp", "GameHost: test"); err != nil {
		t.Fatalf("map: %v", err)
	}
	// Proto comparison is case-insensitive; the router call uses uppercase.
	if !m.IsMapped(25565, "tcp") || !m.IsMapped(25565, "TCP") {
		t.Fatal("expected port mapped")
	}
	if !f.added["TCP:25565"] {
		t.Fatalf("AddPortMapping not called with uppercased proto: %v", f.added)
	}
	if m.ExternalIP() != "203.0.113.7" {
		t.Fatalf("external IP: %q", m.ExternalIP())
	}

	if err := m.Unmap(ctx, 25565, "tcp"); err != nil {
		t.Fatalf("unmap: %v", err)
	}
	if m.IsMapped(25565, "tcp") {
		t.Fatal("expected port unmapped")
	}
	if !f.deleted["TCP:25565"] {
		t.Fatalf("DeletePortMapping not called: %v", f.deleted)
	}
}

func TestProbeNoRouter(t *testing.T) {
	st := newWithClient(nil, "").Probe(context.Background())
	if st.UPnPAvailable {
		t.Fatal("expected UPnP unavailable with no client")
	}
	if st.Message == "" {
		t.Fatal("expected a message")
	}
}

func TestProbeAvailable(t *testing.T) {
	st := newWithClient(newFakeIGD(), "203.0.113.7").Probe(context.Background())
	if !st.UPnPAvailable || st.ExternalIP != "203.0.113.7" {
		t.Fatalf("probe: %+v", st)
	}
}

func TestUnmapWithNoRouterIsNoop(t *testing.T) {
	m := newWithClient(nil, "")
	if err := m.Unmap(context.Background(), 25565, "tcp"); err != nil {
		t.Fatalf("unmap with no router should be a no-op, got %v", err)
	}
}
