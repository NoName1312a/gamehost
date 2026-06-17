package tunnel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeSup is an in-memory supervisor so Agent tests never spawn a real frpc.
type fakeSup struct {
	restarts int
	stops    int
	running  bool
	lastCfg  string
}

func (f *fakeSup) restart(cfg string) error {
	f.restarts++
	f.lastCfg = cfg
	f.running = true
	return nil
}
func (f *fakeSup) stop()           { f.stops++; f.running = false }
func (f *fakeSup) isRunning() bool { return f.running }

// cpRecorder is a canned control-plane recording calls; failSlug forces a 500
// for one slug's allocate to exercise best-effort handling.
type cpRecorder struct {
	mu        sync.Mutex
	registers int
	allocates map[string]int
	releases  map[string]int
	failSlug  string
}

func newCP() *cpRecorder {
	return &cpRecorder{allocates: map[string]int{}, releases: map[string]int{}}
}

func (r *cpRecorder) server(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		defer r.mu.Unlock()
		switch req.URL.Path {
		case "/v1/register":
			r.registers++
			_ = json.NewEncoder(w).Encode(map[string]string{"deviceId": "d", "token": "t"})
		case "/v1/allocate":
			var body struct {
				Slug  string    `json:"slug"`
				Ports []PortReq `json:"ports"`
			}
			_ = json.NewDecoder(req.Body).Decode(&body)
			if body.Slug == r.failSlug {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "boom"})
				return
			}
			r.allocates[body.Slug]++
			var proxies []map[string]any
			for i, p := range body.Ports {
				rp := 30000 + i
				proxies = append(proxies, map[string]any{
					"name":       "gn-" + body.Slug + "-" + p.Role,
					"role":       p.Role,
					"proto":      p.Proto,
					"remotePort": rp,
					"address":    body.Slug + ".gn.coderaum.com:" + strconv.Itoa(rp),
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"slug":    body.Slug,
				"secret":  "sec-" + body.Slug,
				"frps":    map[string]string{"addr": "frps.gn.coderaum.com:7000", "token": "ftok"},
				"proxies": proxies,
			})
		case "/v1/release":
			var body struct {
				Slug string `json:"slug"`
			}
			_ = json.NewDecoder(req.Body).Decode(&body)
			r.releases[body.Slug]++
			_ = json.NewEncoder(w).Encode(map[string]int{"released": 1})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestReconcileAddsNoopReleases(t *testing.T) {
	rec := newCP()
	srv := rec.server(t)
	dir := t.TempDir()
	fake := &fakeSup{}
	a := newAgent(NewClient(srv.URL, dir), fake, dir)

	want := []Desired{{
		Slug:       "mc",
		Ports:      []PortReq{{Role: "game", Proto: "udp"}},
		LocalPorts: map[string]int{"game": 25565},
	}}

	// Adds on first want.
	got, err := a.Reconcile(context.Background(), want)
	if err != nil {
		t.Fatalf("reconcile add: %v", err)
	}
	if len(got) != 1 || got["mc"].Slug != "mc" {
		t.Fatalf("want mc allocated, got %+v", got)
	}
	if rec.allocates["mc"] != 1 {
		t.Fatalf("allocate mc = %d, want 1", rec.allocates["mc"])
	}
	if fake.restarts != 1 || !fake.running {
		t.Fatalf("sidecar should restart+run on add: restarts=%d running=%v", fake.restarts, fake.running)
	}
	cfg, _ := os.ReadFile(a.cfgPath)
	if !strings.Contains(string(cfg), "localPort = 25565") || !strings.Contains(string(cfg), "metadatas.gnsecret = \"sec-mc\"") {
		t.Fatalf("config missing local port or secret:\n%s", cfg)
	}
	if addrs := a.Addresses("mc"); len(addrs) != 1 || addrs[0].Address == "" {
		t.Fatalf("addresses not surfaced: %+v", addrs)
	}

	// No-op when unchanged.
	if _, err := a.Reconcile(context.Background(), want); err != nil {
		t.Fatalf("reconcile noop: %v", err)
	}
	if rec.allocates["mc"] != 1 {
		t.Fatalf("re-allocated on unchanged want: %d", rec.allocates["mc"])
	}
	if fake.restarts != 1 {
		t.Fatalf("restarted on unchanged want: %d", fake.restarts)
	}

	// Empty want releases and stops the sidecar.
	if _, err := a.Reconcile(context.Background(), nil); err != nil {
		t.Fatalf("reconcile release: %v", err)
	}
	if rec.releases["mc"] != 1 {
		t.Fatalf("release mc = %d, want 1", rec.releases["mc"])
	}
	if a.Addresses("mc") != nil {
		t.Fatal("addresses should be gone after release")
	}
	if fake.stops == 0 || fake.running {
		t.Fatalf("sidecar should be stopped after release: stops=%d running=%v", fake.stops, fake.running)
	}
}

func TestReconcileBestEffortOnAllocateFailure(t *testing.T) {
	rec := newCP()
	rec.failSlug = "bad"
	srv := rec.server(t)
	dir := t.TempDir()
	fake := &fakeSup{}
	a := newAgent(NewClient(srv.URL, dir), fake, dir)

	want := []Desired{
		{Slug: "good", Ports: []PortReq{{Role: "game", Proto: "udp"}}, LocalPorts: map[string]int{"game": 25565}},
		{Slug: "bad", Ports: []PortReq{{Role: "game", Proto: "udp"}}, LocalPorts: map[string]int{"game": 7777}},
	}
	got, err := a.Reconcile(context.Background(), want)
	if err != nil {
		t.Fatalf("Reconcile must be best-effort, got err: %v", err)
	}
	if _, ok := got["good"]; !ok {
		t.Fatal("good should be allocated despite bad failing")
	}
	if _, ok := got["bad"]; ok {
		t.Fatal("bad should NOT be allocated (its allocate failed)")
	}
	if !fake.running {
		t.Fatal("sidecar should run for the good server")
	}
}
