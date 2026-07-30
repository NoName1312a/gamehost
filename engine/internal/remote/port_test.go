package remote

import "testing"

// oauthLoopbackPort mirrors the port the desktop shell binds for the OAuth
// sign-in redirect (desktop/src/main.rs). It is fixed: the redirect URL is
// allow-listed with Supabase and Discord, so of the two features this is the
// one that cannot move.
const oauthLoopbackPort = 8788

// Remote access used to default to the same port, so switching it on silently
// broke Discord sign-in — and the resulting "port is busy" named neither
// feature. Nothing in the compiler stops the two drifting back together, so
// state it here.
func TestDefaultPortDoesNotCollideWithOAuthLoopback(t *testing.T) {
	if DefaultPort == oauthLoopbackPort {
		t.Fatalf("remote access defaults to %d, the desktop OAuth loopback port; "+
			"enabling remote access will break Discord sign-in", DefaultPort)
	}
}
