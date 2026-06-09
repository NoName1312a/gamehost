package relay

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/leop1/gamehost/engine/internal/secret"
)

func TestSecretEncryptedRoundTripAndLegacy(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)

	// New format: prefix + base64(Protect(key)) reads back as the plain key.
	enc, err := secret.Protect([]byte("my-playit-key"))
	if err != nil {
		t.Fatalf("protect: %v", err)
	}
	if err := os.WriteFile(a.secretFile(), []byte(secretPrefix+base64.StdEncoding.EncodeToString(enc)), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := a.secret(); got != "my-playit-key" {
		t.Errorf("encrypted secret read back as %q, want my-playit-key", got)
	}

	// Legacy plaintext (no prefix) still loads, so existing installs keep working.
	if err := os.WriteFile(a.secretFile(), []byte("legacy-plain-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := a.secret(); got != "legacy-plain-key" {
		t.Errorf("legacy secret read back as %q, want legacy-plain-key", got)
	}
}
