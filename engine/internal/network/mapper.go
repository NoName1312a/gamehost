// Package network implements automatic port forwarding via UPnP (IGD), so a
// game server's port is opened on the user's router without them logging into
// it. Discovery is done once in the background and cached; every operation is
// best-effort — failures are reported, never fatal (the engine works fine on a
// network with no UPnP, it just can't auto-forward).
package network

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"

	ig1 "github.com/huin/goupnp/dcps/internetgateway1"
	ig2 "github.com/huin/goupnp/dcps/internetgateway2"
)

var errNoIGD = errors.New("no UPnP-capable router found")

// igdClient is the slice of an IGD WAN-connection service we use. The generated
// goupnp clients (IGD v1/v2, IP/PPP) all satisfy it.
type igdClient interface {
	GetExternalIPAddress() (string, error)
	AddPortMapping(remoteHost string, externalPort uint16, protocol string, internalPort uint16, internalClient string, enabled bool, description string, leaseDuration uint32) error
	DeletePortMapping(remoteHost string, externalPort uint16, protocol string) error
}

// Status is the UPnP capability report surfaced to the UI.
type Status struct {
	UPnPAvailable bool   `json:"upnpAvailable"`
	ExternalIP    string `json:"externalIP,omitempty"`
	Message       string `json:"message"`
}

// Mapper discovers the router's IGD service(s) and manages port mappings.
//
// A router can expose several WAN-connection services where only some accept
// AddPortMapping (others answer status queries but reject mappings with a UPnP
// 403). So we keep every service that reported an external IP and, on the first
// successful mapping, remember which one works.
type Mapper struct {
	once sync.Once
	done chan struct{} // closed when discovery completes

	mu         sync.Mutex
	clients    []igdClient     // WAN services that answered (may be empty)
	chosen     igdClient       // the one proven to accept mappings; nil until one does
	externalIP string          // public IP, "" if unknown
	active     map[string]bool // "TCP:25565" -> mapped
}

// New returns a Mapper. Discovery is lazy (first use).
func New() *Mapper {
	return &Mapper{done: make(chan struct{}), active: map[string]bool{}}
}

func (m *Mapper) start() {
	m.once.Do(func() {
		go func() {
			cs, ip := discover()
			m.mu.Lock()
			m.clients, m.externalIP = cs, ip
			m.mu.Unlock()
			close(m.done)
		}()
	})
}

// await blocks until discovery finishes or ctx is done; reports whether ready.
func (m *Mapper) await(ctx context.Context) bool {
	m.start()
	select {
	case <-m.done:
		return true
	case <-ctx.Done():
		return false
	}
}

// Probe reports UPnP availability + the public IP, waiting briefly for the
// (cached) discovery to finish.
func (m *Mapper) Probe(ctx context.Context) Status {
	if !m.await(ctx) {
		return Status{Message: "Checking your router for automatic port forwarding…"}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.clients) == 0 {
		return Status{Message: "No UPnP-capable router found — you can forward the port manually."}
	}
	return Status{UPnPAvailable: true, ExternalIP: m.externalIP, Message: "Your router supports automatic port forwarding."}
}

// Map forwards externalPort -> this PC:port on the router (best-effort). It
// tries each discovered service until one accepts the mapping.
func (m *Mapper) Map(ctx context.Context, port int, proto, desc string) error {
	if !m.await(ctx) {
		return ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.clients) == 0 {
		return errNoIGD
	}
	lan := localIP()
	if lan == "" {
		return errors.New("could not determine this PC's local IP")
	}
	proto = strings.ToUpper(proto)

	var lastErr error
	for _, c := range m.candidatesLocked() {
		// Lease 0 = until removed; we remove on stop/delete/shutdown.
		if err := c.AddPortMapping("", uint16(port), proto, uint16(port), lan, true, desc, 0); err == nil {
			m.chosen = c
			m.active[key(proto, port)] = true
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errNoIGD
	}
	return lastErr
}

// Unmap removes a previously added mapping (no-op if there's no router).
func (m *Mapper) Unmap(ctx context.Context, port int, proto string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	proto = strings.ToUpper(proto)
	delete(m.active, key(proto, port))
	var err error
	for _, c := range m.candidatesLocked() {
		if e := c.DeletePortMapping("", uint16(port), proto); e == nil {
			return nil
		} else {
			err = e
		}
	}
	return err
}

// ExternalIP returns the cached public IP ("" until discovery completes). Never
// blocks, so it's safe to call on every status poll.
func (m *Mapper) ExternalIP() string {
	m.start()
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.externalIP
}

// IsMapped reports whether the given port/proto is currently forwarded.
func (m *Mapper) IsMapped(port int, proto string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active[key(strings.ToUpper(proto), port)]
}

// UnmapAll removes every active mapping; called on graceful engine shutdown so
// ports don't linger on the router.
func (m *Mapper) UnmapAll(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.active {
		proto, port := parseKey(k)
		for _, c := range m.candidatesLocked() {
			_ = c.DeletePortMapping("", uint16(port), proto)
		}
		delete(m.active, k)
	}
}

// candidatesLocked returns the proven client if we have one, else all of them.
// Caller holds m.mu.
func (m *Mapper) candidatesLocked() []igdClient {
	if m.chosen != nil {
		return []igdClient{m.chosen}
	}
	return m.clients
}

// discover runs SSDP for the common IGD service variants (v1/v2, IP/PPP) and
// returns every client that reports a valid external IP, plus that IP.
func discover() ([]igdClient, string) {
	var found []igdClient
	found = append(found, widen(must(ig2.NewWANIPConnection2Clients()))...)
	found = append(found, widen(must(ig2.NewWANIPConnection1Clients()))...)
	found = append(found, widen(must(ig2.NewWANPPPConnection1Clients()))...)
	found = append(found, widen(must(ig1.NewWANIPConnection1Clients()))...)
	found = append(found, widen(must(ig1.NewWANPPPConnection1Clients()))...)

	var working []igdClient
	ip := ""
	for _, c := range found {
		got, err := c.GetExternalIPAddress()
		if err != nil {
			continue
		}
		got = strings.TrimSpace(got)
		if net.ParseIP(got) == nil {
			continue
		}
		working = append(working, c)
		if ip == "" {
			ip = got
		}
	}
	return working, ip
}

// must drops the per-device error slice / discovery error from the goupnp
// New*Clients helpers, keeping just the clients.
func must[T any](clients []T, _ []error, _ error) []T { return clients }

// widen converts a slice of a concrete goupnp client type to []igdClient.
func widen[T igdClient](cs []T) []igdClient {
	out := make([]igdClient, len(cs))
	for i, c := range cs {
		out[i] = c
	}
	return out
}

// localIP returns this machine's primary outbound (LAN) IP. The UDP "dial"
// doesn't send anything — it just picks the route the OS would use.
func localIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if a, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return a.IP.String()
	}
	return ""
}

func key(proto string, port int) string { return proto + ":" + strconv.Itoa(port) }

func parseKey(k string) (string, int) {
	i := strings.IndexByte(k, ':')
	if i < 0 {
		return k, 0
	}
	port, _ := strconv.Atoi(k[i+1:])
	return k[:i], port
}
