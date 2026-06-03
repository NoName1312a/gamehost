package relay

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocateViaEnv(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "playit.exe")
	if err := os.WriteFile(bin, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GAMEHOST_PLAYIT", bin)
	if got := locate(); got != bin {
		t.Fatalf("locate via env: got %q want %q", got, bin)
	}

	bogus := filepath.Join(dir, "nope.exe")
	t.Setenv("GAMEHOST_PLAYIT", bogus)
	if locate() == bogus {
		t.Fatal("locate returned a non-existent override")
	}
}

func TestStatusCarriesURLsAndMessage(t *testing.T) {
	t.Setenv("GAMEHOST_PLAYIT", "")
	st := New(t.TempDir()).Status()
	if st.SetupURL == "" || st.DashboardURL == "" {
		t.Fatal("status should carry setup + dashboard URLs")
	}
	if st.Message == "" {
		t.Fatal("status should have a message")
	}
}

func TestRunActionUnknown(t *testing.T) {
	if err := New(t.TempDir()).RunAction(context.Background(), "bogus"); err == nil {
		t.Fatal("expected error for unknown relay action")
	}
}

// TestLinkedReflectsSecretFile checks the linked/secret logic without starting
// the real daemon (we write the secret file directly rather than calling Link).
func TestLinkedReflectsSecretFile(t *testing.T) {
	a := New(t.TempDir())
	if a.Status().Linked {
		t.Fatal("should not be linked before a secret is set")
	}
	if err := os.WriteFile(a.secretFile(), []byte("  test-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !a.Status().Linked {
		t.Fatal("should be linked after the secret file is written")
	}
	if a.secret() != "test-secret" {
		t.Fatalf("secret should be trimmed, got %q", a.secret())
	}
}
