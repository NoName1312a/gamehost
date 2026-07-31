package templates

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// smokeEnv opts a run in to the network. Off by default so `go test ./...`
// stays offline and deterministic; CI sets it.
const smokeEnv = "GAMEHOST_REGISTRY_SMOKE"

// TestBundledImagesArePullable asks every bundled template's registry whether
// that exact image:tag can still be pulled.
//
// A game whose image reference has rotted is indistinguishable from a working
// one until a user picks it: nothing validates templates at load time, so the
// first symptom is a failed container create on someone's PC. That is how
// Veloren shipped broken in 0.6.9 — florianpiesche/veloren-server never had a
// `latest` tag, only dated `nightly-*` ones, so the pull 404'd for everybody
// who tried it.
//
// This only proves the image resolves. It does not prove the game boots, the
// env var names are right, or the ports are the ones it listens on.
func TestBundledImagesArePullable(t *testing.T) {
	if os.Getenv(smokeEnv) == "" {
		t.Skipf("network smoke test; set %s=1 to run", smokeEnv)
	}

	reg := NewRegistry(filepath.Join("..", "..", "..", "templates"))
	if err := reg.Load(); err != nil {
		t.Fatalf("load bundled templates: %v", err)
	}
	all := reg.List()
	if len(all) == 0 {
		t.Fatal("no templates loaded — nothing was checked")
	}
	t.Logf("checking %d template images", len(all))

	client := &http.Client{Timeout: 30 * time.Second}
	for _, tpl := range all {
		t.Run(tpl.ID, func(t *testing.T) {
			t.Parallel()

			ref, err := ParseImageRef(tpl.Image)
			if err != nil {
				t.Fatalf("template %q has an unparseable image %q: %v", tpl.ID, tpl.Image, err)
			}
			if err := manifestExists(client, ref); err != nil {
				t.Errorf("template %q image %q is not pullable: %v", tpl.ID, tpl.Image, err)
			}
		})
	}
}

// manifestAccept lists every manifest media type a pull may encounter. Omitting
// the OCI types makes some registries answer 404 for images that exist.
var manifestAccept = strings.Join([]string{
	"application/vnd.docker.distribution.manifest.v2+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.oci.image.manifest.v1+json",
	"application/vnd.oci.image.index.v1+json",
}, ",")

// manifestExists performs the pull handshake without downloading any layers:
// request the manifest, answer the registry's auth challenge if it sends one,
// and retry once with the token.
func manifestExists(client *http.Client, ref ImageRef) error {
	resp, err := headManifest(client, ref, "")
	if err != nil {
		return err
	}
	challenge := resp.Header.Get("WWW-Authenticate")
	status := resp.StatusCode
	resp.Body.Close()

	if status == http.StatusUnauthorized && strings.HasPrefix(challenge, "Bearer ") {
		token, err := fetchToken(client, challenge)
		if err != nil {
			return fmt.Errorf("registry auth: %w", err)
		}
		resp, err = headManifest(client, ref, token)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		status = resp.StatusCode
	}

	switch {
	case status == http.StatusOK:
		return nil
	case status == http.StatusNotFound:
		return fmt.Errorf("tag %q does not exist in %s (HTTP 404)", ref.Tag, ref.Repository)
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP %d: %s", status, strings.TrimSpace(string(body)))
	}
}

func headManifest(client *http.Client, ref ImageRef, token string) (*http.Response, error) {
	// GET, not HEAD: some registries answer HEAD with 405 even for tags they serve.
	req, err := http.NewRequest(http.MethodGet, ref.ManifestURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", manifestAccept)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reach %s: %w", ref.Registry, err)
	}
	return resp, nil
}

// fetchToken answers a Bearer challenge by asking the realm it names for an
// anonymous pull token. Parsing the challenge rather than hard-coding each
// registry's auth host is what lets Docker Hub and GitLab share one path.
func fetchToken(client *http.Client, challenge string) (string, error) {
	params := parseChallenge(strings.TrimPrefix(challenge, "Bearer "))
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("challenge has no realm: %q", challenge)
	}

	u, err := url.Parse(realm)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for _, k := range []string{"service", "scope"} {
		if v := params[k]; v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()

	resp, err := client.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned HTTP %d", resp.StatusCode)
	}

	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.Token != "" {
		return body.Token, nil
	}
	if body.AccessToken != "" {
		return body.AccessToken, nil
	}
	return "", fmt.Errorf("token endpoint returned no token")
}

// parseChallenge splits a `key="value",key="value"` auth challenge.
func parseChallenge(s string) map[string]string {
	out := map[string]string{}
	for _, part := range splitUnquoted(s) {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		out[k] = strings.Trim(v, `"`)
	}
	return out
}

// splitUnquoted splits on commas that are not inside quotes — a scope value
// legitimately contains commas ("pull,push").
func splitUnquoted(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuotes := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			cur.WriteRune(r)
		case r == ',' && !inQuotes:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}
