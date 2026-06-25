package main

// defaultTunnelURL is the GameNest relay control-plane, baked in so the tunnel
// is on by default. (Owner: confirm the exact production URL at release.)
const defaultTunnelURL = "https://cp.coderaum.com"

// resolveTunnelURL decides the control-plane URL the tunnel agent uses.
// GAMEHOST_TUNNEL_DISABLE=<non-empty> forces the tunnel off (returns ""), the
// emergency valve. Otherwise GAMEHOST_TUNNEL_URL overrides the baked default.
func resolveTunnelURL(getenv func(string) string) string {
	if getenv("GAMEHOST_TUNNEL_DISABLE") != "" {
		return ""
	}
	if u := getenv("GAMEHOST_TUNNEL_URL"); u != "" {
		return u
	}
	return defaultTunnelURL
}
