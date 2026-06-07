package tlscert

import (
	"bytes"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureGeneratesAndReloads(t *testing.T) {
	dir := t.TempDir()

	c1, err := Ensure(dir)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if len(c1.Certificate) == 0 {
		t.Fatal("no certificate generated")
	}
	if _, err := os.Stat(filepath.Join(dir, "tls-cert.pem")); err != nil {
		t.Errorf("cert file not persisted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tls-key.pem")); err != nil {
		t.Errorf("key file not persisted: %v", err)
	}

	// The leaf must be parseable and cover localhost so a local browser test works.
	leaf, err := x509.ParseCertificate(c1.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	found := false
	for _, d := range leaf.DNSNames {
		if d == "localhost" {
			found = true
		}
	}
	if !found {
		t.Errorf("cert SANs %v do not include localhost", leaf.DNSNames)
	}

	// Idempotent: a second call reuses the persisted cert, not a fresh one.
	c2, err := Ensure(dir)
	if err != nil {
		t.Fatalf("ensure2: %v", err)
	}
	if !bytes.Equal(c1.Certificate[0], c2.Certificate[0]) {
		t.Error("cert changed on reload; it should be stable across restarts")
	}
}
