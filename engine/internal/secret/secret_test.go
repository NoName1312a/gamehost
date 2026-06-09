package secret

import (
	"bytes"
	"testing"
)

func TestProtectUnprotectRoundTrip(t *testing.T) {
	in := []byte("super-secret-playit-key-12345")
	enc, err := Protect(in)
	if err != nil {
		t.Fatalf("protect: %v", err)
	}
	dec, err := Unprotect(enc)
	if err != nil {
		t.Fatalf("unprotect: %v", err)
	}
	if !bytes.Equal(dec, in) {
		t.Errorf("round-trip mismatch: got %q want %q", dec, in)
	}
}

func TestProtectEmpty(t *testing.T) {
	enc, err := Protect(nil)
	if err != nil {
		t.Fatalf("protect empty: %v", err)
	}
	dec, err := Unprotect(enc)
	if err != nil {
		t.Fatalf("unprotect empty: %v", err)
	}
	if len(dec) != 0 {
		t.Errorf("empty round-trip should be empty, got %q", dec)
	}
}
