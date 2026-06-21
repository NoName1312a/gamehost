// Package account links the engine to a GameNest platform account and fetches
// entitlement tokens. It persists the device credential DPAPI-protected on disk
// (same gh1: scheme as the tunnel token) and exposes a small HTTP client for the
// platform's /api/link and /api/entitlement endpoints.
package account

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/leop1/gamehost/engine/internal/secret"
)

// secretPrefix marks the DPAPI-encrypted, base64 on-disk token format.
// Identical to the tunnel package's convention.
const secretPrefix = "gh1:"

// Store holds the platform URL and the linked device credential.
type Store struct {
	platformURL string
	hc          *http.Client
	dataDir     string

	mu    sync.Mutex // guards token read/write and lazy load
	token string     // in-memory cache; "" means not yet loaded or not linked
}

// New returns a Store that persists its credential under dataDir and calls
// platformURL for link/entitlement operations.
func New(dataDir, platformURL string) *Store {
	return &Store{
		platformURL: strings.TrimRight(platformURL, "/"),
		hc:          &http.Client{Timeout: 15 * time.Second},
		dataDir:     dataDir,
	}
}

func (s *Store) tokenFile() string { return filepath.Join(s.dataDir, "account-token") }

// loadToken reads the device token from disk, decrypting the DPAPI format.
// Returns "" if absent or unreadable.
func (s *Store) loadToken() string {
	b, err := os.ReadFile(s.tokenFile())
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(b))
	rest, ok := strings.CutPrefix(content, secretPrefix)
	if !ok {
		// legacy plaintext fall-through (shouldn't happen for this package, but
		// mirrors the tunnel client's defensive read)
		return content
	}
	enc, err := base64.StdEncoding.DecodeString(rest)
	if err != nil {
		return ""
	}
	dec, err := secret.Unprotect(enc)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(dec))
}

// storeToken writes the device token DPAPI-encrypted (gh1: + base64).
func (s *Store) storeToken(token string) error {
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return err
	}
	enc, err := secret.Protect([]byte(token))
	if err != nil {
		return err
	}
	data := secretPrefix + base64.StdEncoding.EncodeToString(enc)
	return os.WriteFile(s.tokenFile(), []byte(data), 0o600)
}

// deviceToken returns the cached in-memory token, loading lazily from disk on
// first call. Returns "" if no credential is stored.
func (s *Store) deviceToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" {
		return s.token
	}
	s.token = s.loadToken()
	return s.token
}

// Linked reports whether a device credential is stored (loads lazily from disk).
func (s *Store) Linked() bool {
	return s.deviceToken() != ""
}

// Link exchanges a user-supplied code for a device token from the platform and
// stores it DPAPI-protected. On success Linked() will return true.
func (s *Store) Link(ctx context.Context, code string) error {
	var resp struct {
		DeviceToken string `json:"deviceToken"`
	}
	if err := s.doJSON(ctx, "/api/link", "", map[string]string{"code": code}, &resp); err != nil {
		return err
	}
	if resp.DeviceToken == "" {
		return fmt.Errorf("platform /api/link: response missing deviceToken")
	}
	if err := s.storeToken(resp.DeviceToken); err != nil {
		return fmt.Errorf("persist device token: %w", err)
	}
	s.mu.Lock()
	s.token = resp.DeviceToken
	s.mu.Unlock()
	return nil
}

// Unlink deletes the stored credential. Linked() will return false afterwards.
func (s *Store) Unlink() error {
	s.mu.Lock()
	s.token = ""
	s.mu.Unlock()
	err := os.Remove(s.tokenFile())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Entitlement fetches an entitlement token for slug from the platform,
// authenticating with the stored device token. Returns the opaque token string
// that can be forwarded to the relay allocate call.
func (s *Store) Entitlement(ctx context.Context, slug string) (string, error) {
	tok := s.deviceToken()
	if tok == "" {
		return "", fmt.Errorf("account: not linked — call Link first")
	}
	var resp struct {
		Token string `json:"token"`
		Exp   int64  `json:"exp"`
	}
	if err := s.doJSON(ctx, "/api/entitlement", tok, map[string]string{"slug": slug}, &resp); err != nil {
		return "", err
	}
	if resp.Token == "" {
		return "", fmt.Errorf("platform /api/entitlement: response missing token")
	}
	return resp.Token, nil
}

// doJSON performs a JSON POST to the platform. A non-empty bearer is sent as
// Authorization; out (if non-nil) receives the decoded response body.
func (s *Store) doJSON(ctx context.Context, path, bearer string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.platformURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := s.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return fmt.Errorf("platform POST %s: %s (%d)", path, e.Error, resp.StatusCode)
		}
		return fmt.Errorf("platform POST %s: status %d", path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
