// Package license verifies Ed25519-signed license keys offline against an
// embedded public key, and persists the operator's key. The matching private
// key signs keys (kept by the vendor, never in the repo) — see cmd/licensegen.
//
// Key format: base64url(payloadJSON) + "." + base64url(signature).
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// License is the signed entitlement payload.
type License struct {
	Email   string `json:"email"`
	Tier    string `json:"tier"`          // "pro"
	Expires int64  `json:"exp,omitempty"` // unix seconds; 0 = perpetual
}

// IsPro reports whether this license grants Pro at time now.
func (l License) IsPro(now time.Time) bool {
	if l.Tier != "pro" {
		return false
	}
	return l.Expires == 0 || now.Unix() <= l.Expires
}

var b64 = base64.RawURLEncoding

// Sign produces a license key by signing the payload with priv.
func Sign(priv ed25519.PrivateKey, lic License) (string, error) {
	payload, err := json.Marshal(lic)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(priv, payload)
	return b64.EncodeToString(payload) + "." + b64.EncodeToString(sig), nil
}

// VerifyWith verifies key against pub and returns the decoded License.
func VerifyWith(pub ed25519.PublicKey, key string) (License, error) {
	parts := strings.Split(strings.TrimSpace(key), ".")
	if len(parts) != 2 {
		return License{}, errors.New("malformed license key")
	}
	payload, err := b64.DecodeString(parts[0])
	if err != nil {
		return License{}, errors.New("malformed license payload")
	}
	sig, err := b64.DecodeString(parts[1])
	if err != nil {
		return License{}, errors.New("malformed license signature")
	}
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, payload, sig) {
		return License{}, errors.New("license signature is invalid")
	}
	var lic License
	if err := json.Unmarshal(payload, &lic); err != nil {
		return License{}, errors.New("malformed license payload")
	}
	return lic, nil
}

// Verify verifies key against the embedded production public key.
func Verify(key string) (License, error) {
	return VerifyWith(EmbeddedPublicKey(), key)
}

// Store persists the operator's license key and exposes its entitlements.
type Store struct {
	path string
	pub  ed25519.PublicKey

	mu  sync.RWMutex
	lic License
	ok  bool
}

// NewStore loads any persisted key from <dataDir>/license.key and verifies it
// against pub (pass EmbeddedPublicKey() in production).
func NewStore(dataDir string, pub ed25519.PublicKey) *Store {
	s := &Store{path: filepath.Join(dataDir, "license.key"), pub: pub}
	if b, err := os.ReadFile(s.path); err == nil {
		if lic, err := VerifyWith(pub, string(b)); err == nil {
			s.lic, s.ok = lic, true
		}
	}
	return s
}

// Set validates a license key and, if valid, persists it.
func (s *Store) Set(key string) error {
	lic, err := VerifyWith(s.pub, key)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, []byte(strings.TrimSpace(key)), 0o600); err != nil {
		return err
	}
	s.mu.Lock()
	s.lic, s.ok = lic, true
	s.mu.Unlock()
	return nil
}

// Clear removes the stored license (revert to free).
func (s *Store) Clear() error {
	s.mu.Lock()
	s.lic, s.ok = License{}, false
	s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// IsPro reports whether a valid, unexpired Pro license is loaded.
func (s *Store) IsPro() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ok && s.lic.IsPro(time.Now())
}

// Info returns the loaded license's tier/email and whether it's valid Pro.
func (s *Store) Info() (tier, email string, pro bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ok {
		return "free", "", false
	}
	return s.lic.Tier, s.lic.Email, s.lic.IsPro(time.Now())
}
