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

	// A bogus override must not be returned (it doesn't exist).
	bogus := filepath.Join(dir, "nope.exe")
	t.Setenv("GAMEHOST_PLAYIT", bogus)
	if locate() == bogus {
		t.Fatal("locate returned a non-existent override")
	}
}

func TestStatusCarriesURLsAndMessage(t *testing.T) {
	t.Setenv("GAMEHOST_PLAYIT", "")
	st := New().Status()
	if st.SetupURL == "" || st.DashboardURL == "" {
		t.Fatal("status should carry setup + dashboard URLs")
	}
	if st.Message == "" {
		t.Fatal("status should have a message")
	}
}

func TestRunActionUnknown(t *testing.T) {
	if err := New().RunAction(context.Background(), "bogus"); err == nil {
		t.Fatal("expected error for unknown relay action")
	}
}

func TestSecretPathNonEmpty(t *testing.T) {
	if secretPath() == "" {
		t.Fatal("secretPath should not be empty")
	}
}
