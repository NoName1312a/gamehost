// Package auth provides password-based operator authentication and session
// tokens for the engine. It supports multiple named accounts with roles; the
// HTTP layer trusts loopback (the local desktop user = owner) and enforces auth
// only for non-loopback (remote) access. Single-password installs migrate to a
// lone "owner" account.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

// Roles. Only the owner can manage users; admin/operator can manage servers.
const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	ownerName    = "owner"
)

var userNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{1,31}$`)

type credential struct {
	Algo    string `json:"algo"`
	Salt    string `json:"salt"`
	Hash    string `json:"hash"`
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
}

type user struct {
	Username string     `json:"username"`
	Role     string     `json:"role"`
	Cred     credential `json:"cred"`
}

type authFile struct {
	Users []user `json:"users"`
}

type session struct {
	username string
	expiry   time.Time
}

// UserInfo is the public view of an account (no secrets).
type UserInfo struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Store holds accounts (persisted) and sessions (in-memory).
type Store struct {
	path     string
	mu       sync.RWMutex
	users    map[string]*user
	sessions map[string]session
}

// New loads accounts from <dataDir>/auth.json, migrating a legacy single
// credential to a lone owner account.
func New(dataDir string) (*Store, error) {
	s := &Store{
		path:     filepath.Join(dataDir, "auth.json"),
		users:    map[string]*user{},
		sessions: map[string]session{},
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	var af authFile
	if err := json.Unmarshal(b, &af); err == nil && len(af.Users) > 0 {
		for i := range af.Users {
			u := af.Users[i]
			s.users[u.Username] = &u
		}
		return s, nil
	}
	// Legacy single-credential format -> migrate to an owner account.
	var legacy credential
	if err := json.Unmarshal(b, &legacy); err == nil && legacy.Hash != "" {
		s.users[ownerName] = &user{Username: ownerName, Role: RoleOwner, Cred: legacy}
		_ = s.persistLocked()
	}
	return s, nil
}

// HasPassword reports whether any account exists (i.e. a password is set).
func (s *Store) HasPassword() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users) > 0
}

// SetPassword sets the owner's password, creating the owner on first run.
func (s *Store) SetPassword(pw string) error {
	cred, err := hashPassword(pw)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if u := s.users[ownerName]; u != nil {
		u.Cred = cred
	} else {
		s.users[ownerName] = &user{Username: ownerName, Role: RoleOwner, Cred: cred}
	}
	return s.persistLocked()
}

// Verify checks a password against the owner account.
func (s *Store) Verify(pw string) bool { return s.VerifyUser(ownerName, pw) }

// VerifyUser checks a password against the named account.
func (s *Store) VerifyUser(username, pw string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u := s.users[username]
	return u != nil && verifyCred(u.Cred, pw)
}

// AddUser creates a non-owner account.
func (s *Store) AddUser(username, pw, role string) error {
	username = strings.TrimSpace(username)
	if !userNameRe.MatchString(username) {
		return errors.New("username must be 2-32 letters, numbers, or . _ -")
	}
	if username == ownerName {
		return errors.New("that username is reserved")
	}
	if role != RoleAdmin && role != RoleOperator {
		return errors.New("role must be admin or operator")
	}
	if len(pw) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	cred, err := hashPassword(pw)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[username]; exists {
		return errors.New("a user with that name already exists")
	}
	s.users[username] = &user{Username: username, Role: role, Cred: cred}
	return s.persistLocked()
}

// DeleteUser removes a non-owner account.
func (s *Store) DeleteUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.users[username]
	if u == nil {
		return errors.New("user not found")
	}
	if u.Role == RoleOwner {
		return errors.New("the owner account can't be deleted")
	}
	delete(s.users, username)
	return s.persistLocked()
}

// ListUsers returns all accounts (no secrets), sorted by username.
func (s *Store) ListUsers() []UserInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UserInfo, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, UserInfo{Username: u.Username, Role: u.Role})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out
}

// UserRole returns a user's role.
func (s *Store) UserRole(username string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u := s.users[username]
	if u == nil {
		return "", false
	}
	return u.Role, true
}

// CreateSession mints a session for the owner.
func (s *Store) CreateSession(ttl time.Duration) string { return s.CreateSessionFor(ownerName, ttl) }

// CreateSessionFor mints a session for a specific account.
func (s *Store) CreateSessionFor(username string, ttl time.Duration) string {
	tok := randToken()
	s.mu.Lock()
	s.sessions[tok] = session{username: username, expiry: time.Now().Add(ttl)}
	s.mu.Unlock()
	return tok
}

// ValidateSession reports whether the token is a live session.
func (s *Store) ValidateSession(tok string) bool {
	_, ok := s.SessionUsername(tok)
	return ok
}

// SessionUsername returns the account a live session belongs to.
func (s *Store) SessionUsername(tok string) (string, bool) {
	if tok == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[tok]
	if !ok {
		return "", false
	}
	if time.Now().After(sess.expiry) {
		delete(s.sessions, tok)
		return "", false
	}
	return sess.username, true
}

// DeleteSession invalidates a session token (logout).
func (s *Store) DeleteSession(tok string) {
	s.mu.Lock()
	delete(s.sessions, tok)
	s.mu.Unlock()
}

// persistLocked writes the accounts file. Caller holds s.mu (or is New).
func (s *Store) persistLocked() error {
	af := authFile{}
	for _, u := range s.users {
		af.Users = append(af.Users, *u)
	}
	sort.Slice(af.Users, func(i, j int) bool { return af.Users[i].Username < af.Users[j].Username })
	b, err := json.MarshalIndent(af, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func hashPassword(pw string) (credential, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return credential{}, err
	}
	hash := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return credential{
		Algo:    "argon2id",
		Salt:    base64.StdEncoding.EncodeToString(salt),
		Hash:    base64.StdEncoding.EncodeToString(hash),
		Time:    argonTime,
		Memory:  argonMemory,
		Threads: argonThreads,
	}, nil
}

func verifyCred(c credential, pw string) bool {
	if c.Hash == "" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(c.Salt)
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(c.Hash)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, c.Time, c.Memory, c.Threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func randToken() string {
	b := make([]byte, tokenLen)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
