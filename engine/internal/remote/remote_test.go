package remote

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestStatePersists(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, "127.0.0.1")
	if c.Status().Enabled {
		t.Fatal("fresh controller should be disabled")
	}
	c.SetHandler(http.NewServeMux())
	if err := c.Enable(0); err != nil {
		t.Fatalf("enable: %v", err)
	}
	defer c.Shutdown(context.Background())
	if !c.Status().Enabled {
		t.Error("should be enabled after Enable")
	}

	// A fresh controller over the same dir loads enabled=true.
	if c2 := New(dir, "127.0.0.1"); !c2.Status().Enabled {
		t.Error("enabled state did not persist across restart")
	}
}

func TestServesOverTLSWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	c := New(dir, "127.0.0.1")
	c.SetHandler(mux)
	if err := c.Enable(0); err != nil { // port 0 -> ephemeral
		t.Fatalf("enable: %v", err)
	}
	defer c.Shutdown(context.Background())

	addr := c.Status().Addr
	if addr == "" {
		t.Fatal("Status().Addr empty while running")
	}
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		Timeout:   3 * time.Second,
	}
	var resp *http.Response
	var err error
	for i := 0; i < 20; i++ {
		resp, err = client.Get("https://" + addr + "/api/health")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("https get: %v", err)
	}
	defer resp.Body.Close()
	if b, _ := io.ReadAll(resp.Body); string(b) != "ok" {
		t.Errorf("body = %q, want ok", b)
	}

	// Disable stops serving.
	if err := c.Disable(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if c.Status().Enabled {
		t.Error("should be disabled after Disable")
	}
}
