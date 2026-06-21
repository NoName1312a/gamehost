package tunnel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAllocateIncludesEntitlementWhenSet(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/register":
			_ = json.NewEncoder(w).Encode(map[string]string{"deviceId": "d", "token": "tok"})
		case "/v1/allocate":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"slug": gotBody["slug"], "secret": "s",
				"frps":    map[string]string{"addr": "a", "token": "t"},
				"proxies": []any{},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// NewClient(baseURL, dataDir) — real signature from client.go
	c := NewClient(srv.URL, t.TempDir())

	// Paid path: entitlement must appear in the request body.
	if _, err := c.Allocate(context.Background(), "alice", []PortReq{{Role: "game", Proto: "tcp"}}, "ent.tok"); err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if gotBody["entitlement"] != "ent.tok" {
		t.Fatalf("entitlement not forwarded: %v", gotBody["entitlement"])
	}

	// Free path: entitlement must be absent from the request body.
	gotBody = nil
	if _, err := c.Allocate(context.Background(), "gn-abcd12", []PortReq{{Role: "game", Proto: "tcp"}}, ""); err != nil {
		t.Fatalf("free allocate: %v", err)
	}
	if _, present := gotBody["entitlement"]; present {
		t.Fatalf("entitlement must be absent for the free path, got %v", gotBody["entitlement"])
	}
}

// canned control-plane handler used by the client tests.
func cpHandler(registerCalls *int, gotBearer *string, gotReleaseBody *string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/register":
			*registerCalls++
			_ = json.NewEncoder(w).Encode(map[string]string{"deviceId": "dev123", "token": "tok-abc"})
		case "/v1/allocate":
			*gotBearer = r.Header.Get("Authorization")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"slug":   "myserver",
				"secret": "s3cr3t",
				"frps":   map[string]string{"addr": "frps.example:7000", "token": "frps-tok"},
				"proxies": []map[string]any{
					{"name": "gn-myserver-game", "role": "game", "proto": "udp", "remotePort": 30001, "address": "myserver.gn.coderaum.com:30001"},
				},
			})
		case "/v1/release":
			b, _ := io.ReadAll(r.Body)
			*gotReleaseBody = string(b)
			_ = json.NewEncoder(w).Encode(map[string]int{"released": 1})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestClientRegistersOnceAndPersistsToken(t *testing.T) {
	var registerCalls int
	var gotBearer, gotReleaseBody string
	srv := httptest.NewServer(cpHandler(&registerCalls, &gotBearer, &gotReleaseBody))
	defer srv.Close()

	dir := t.TempDir()

	// First client registers and stores the token.
	c1 := NewClient(srv.URL, dir)
	alloc, err := c1.Allocate(context.Background(), "myserver", []PortReq{{Role: "game", Proto: "udp"}}, "")
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}

	if gotBearer != "Bearer tok-abc" {
		t.Fatalf("bearer = %q, want %q", gotBearer, "Bearer tok-abc")
	}
	if alloc.Slug != "myserver" || alloc.Secret != "s3cr3t" {
		t.Fatalf("slug/secret parsed wrong: %+v", alloc)
	}
	if alloc.FrpsAddr != "frps.example:7000" || alloc.FrpsToken != "frps-tok" {
		t.Fatalf("frps parsed wrong: %+v", alloc)
	}
	if len(alloc.Proxies) != 1 {
		t.Fatalf("want 1 proxy, got %d", len(alloc.Proxies))
	}
	p := alloc.Proxies[0]
	if p.Name != "gn-myserver-game" || p.Role != "game" || p.Proto != "udp" ||
		p.RemotePort != 30001 || p.Address != "myserver.gn.coderaum.com:30001" {
		t.Fatalf("proxy parsed wrong: %+v", p)
	}

	// Token persisted with the DPAPI prefix (not plaintext).
	raw, err := os.ReadFile(c1.tokenFile())
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	stored := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(stored, secretPrefix) {
		t.Fatalf("token not stored with %q prefix: %q", secretPrefix, stored)
	}
	if strings.Contains(stored, "tok-abc") {
		t.Fatalf("plaintext token leaked to disk: %q", stored)
	}

	// A fresh client over the same dataDir reuses the persisted token:
	// no second register, and the bearer still flows.
	c2 := NewClient(srv.URL, dir)
	if err := c2.Release(context.Background(), "myserver"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if registerCalls != 1 {
		t.Fatalf("register called %d times, want exactly 1 (token should persist across clients)", registerCalls)
	}
	if !strings.Contains(gotReleaseBody, `"slug":"myserver"`) {
		t.Fatalf("release body missing slug: %q", gotReleaseBody)
	}
}
