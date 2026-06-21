package account

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLinkThenEntitlement(t *testing.T) {
	var gotAuth, gotSlug string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/link":
			var b map[string]string
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["code"] != "CODE123" {
				w.WriteHeader(400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"deviceToken": "dev-tok"})
		case "/api/entitlement":
			gotAuth = r.Header.Get("Authorization")
			var b map[string]string
			_ = json.NewDecoder(r.Body).Decode(&b)
			gotSlug = b["slug"]
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "ent.tok", "exp": 123})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	s := New(t.TempDir(), srv.URL)
	if s.Linked() {
		t.Fatal("fresh store must not be linked")
	}
	if err := s.Link(context.Background(), "CODE123"); err != nil {
		t.Fatalf("link: %v", err)
	}
	if !s.Linked() {
		t.Fatal("should be linked after Link")
	}
	tok, err := s.Entitlement(context.Background(), "alice")
	if err != nil {
		t.Fatalf("entitlement: %v", err)
	}
	if tok != "ent.tok" || gotSlug != "alice" || gotAuth != "Bearer dev-tok" {
		t.Fatalf("bad entitlement flow: tok=%q slug=%q auth=%q", tok, gotSlug, gotAuth)
	}
	if err := s.Unlink(); err != nil || s.Linked() {
		t.Fatalf("unlink failed: err=%v linked=%v", err, s.Linked())
	}
}
