// Package auth provides password-based operator authentication and session
// tokens for the engine. It is intentionally small: a single operator password
// (argon2id-hashed, persisted) plus in-memory session tokens. The HTTP layer
// trusts loopback and only enforces this for non-loopback (remote) access.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters (OWASP-ish defaults: 64 MiB, 1 pass, 4 lanes).
const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
	tokenLen     = 32
)

// credential is the persisted password hash and its parameters.
type credential struct {
	Algo    string `json:"algo"`
	Salt    string `json:"salt"` // base64
	Hash    string `json:"hash"` // base64
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
}

// Store holds the operator credential (persisted) and active sessions (memory).
type Store struct {
	path string

	mu       sync.RWMutex
	cred     credential
	sessions map[string]time.Time // token -> expiry
}

// New loads the credential from <dataDir>/auth.json (if present) and returns a
// ready Store.
func New(dataDir string) (*Store, error) {
	s := &Store{
		path:     filepath.Join(dataDir, "auth.json"),
		sessions: map[string]time.Time{},
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &s.cred); err != nil {
		return nil, err
	}
	return s, nil
}

// HasPassword reports whether an operator password has been set.
func (s *Store) HasPassword() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cred.Hash != ""
}

// SetPassword hashes and persists a new operator password.
func (s *Store) SetPassword(pw string) error {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hash := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	cred := credential{
		Algo:    "argon2id",
		Salt:    base64.StdEncoding.EncodeToString(salt),
		Hash:    base64.StdEncoding.EncodeToString(hash),
		Time:    argonTime,
		Memory:  argonMemory,
		Threads: argonThreads,
	}
	b, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.mu.Lock()
	s.cred = cred
	s.mu.Unlock()
	return nil
}

// Verify reports whether pw matches the stored password (false if none set).
func (s *Store) Verify(pw string) bool {
	s.mu.RLock()
	cred := s.cred
	s.mu.RUnlock()
	if cred.Hash == "" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(cred.Salt)
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(cred.Hash)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, cred.Time, cred.Memory, cred.Threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// CreateSession mints a random session token valid for ttl and returns it.
func (s *Store) CreateSession(ttl time.Duration) string {
	tok := randToken()
	s.mu.Lock()
	s.sessions[tok] = time.Now().Add(ttl)
	s.mu.Unlock()
	return tok
}

// ValidateSession reports whether the token is a live (unexpired) session.
func (s *Store) ValidateSession(tok string) bool {
	if tok == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.sessions[tok]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.sessions, tok)
		return false
	}
	return true
}

// DeleteSession invalidates a session token (logout).
func (s *Store) DeleteSession(tok string) {
	s.mu.Lock()
	delete(s.sessions, tok)
	s.mu.Unlock()
}

func randToken() string {
	b := make([]byte, tokenLen)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
