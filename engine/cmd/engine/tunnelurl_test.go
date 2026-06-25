package main

import "testing"

func TestResolveTunnelURL(t *testing.T) {
	const def = defaultTunnelURL
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"default when unset", map[string]string{}, def},
		{"env override", map[string]string{"GAMEHOST_TUNNEL_URL": "https://example.test"}, "https://example.test"},
		{"disabled valve", map[string]string{"GAMEHOST_TUNNEL_DISABLE": "1"}, ""},
		{"disabled beats override", map[string]string{"GAMEHOST_TUNNEL_DISABLE": "1", "GAMEHOST_TUNNEL_URL": "https://x"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			get := func(k string) string { return c.env[k] }
			if got := resolveTunnelURL(get); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
