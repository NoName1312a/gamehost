package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func testKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return pub, priv
}

func TestSignVerifyRoundTrip(t *testing.T) {
	pub, priv := testKeypair(t)
	lic := License{Email: "a@b.com", Tier: "pro"}
	key, err := Sign(priv, lic)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	got, err := VerifyWith(pub, key)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Email != "a@b.com" || got.Tier != "pro" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestVerifyRejectsTamperedKey(t *testing.T) {
	pub, priv := testKeypair(t)
	key, _ := Sign(priv, License{Email: "a@b.com", Tier: "pro"})
	// Flip a character in the payload.
	bad := "x" + key[1:]
	if _, err := VerifyWith(pub, bad); err == nil {
		t.Error("tampered license key must fail verification")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, priv := testKeypair(t)
	otherPub, _ := testKeypair(t)
	key, _ := Sign(priv, License{Email: "a@b.com", Tier: "pro"})
	if _, err := VerifyWith(otherPub, key); err == nil {
		t.Error("license signed by a different key must not verify")
	}
}

func TestIsProRespectsExpiry(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	if !(License{Tier: "pro"}).IsPro(now) {
		t.Error("perpetual pro license should be pro")
	}
	if (License{Tier: "pro", Expires: now.Unix() - 1}).IsPro(now) {
		t.Error("expired license must not be pro")
	}
	if (License{Tier: "free"}).IsPro(now) {
		t.Error("free tier is never pro")
	}
}

func TestStorePersistsAndGates(t *testing.T) {
	pub, priv := testKeypair(t)
	dir := t.TempDir()

	s := NewStore(dir, pub)
	if s.IsPro() {
		t.Fatal("a fresh store with no license must be free")
	}
	key, _ := Sign(priv, License{Email: "a@b.com", Tier: "pro"})
	if err := s.Set(key); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !s.IsPro() {
		t.Error("store should be pro after setting a valid pro key")
	}
	// Persists across reopen.
	if s2 := NewStore(dir, pub); !s2.IsPro() {
		t.Error("license did not persist across reopen")
	}
}

func TestStoreRejectsBadKey(t *testing.T) {
	pub, _ := testKeypair(t)
	s := NewStore(t.TempDir(), pub)
	if err := s.Set("not-a-real-key"); err == nil {
		t.Error("Set must reject an invalid license key")
	}
	if s.IsPro() {
		t.Error("store must remain free after a rejected key")
	}
}
