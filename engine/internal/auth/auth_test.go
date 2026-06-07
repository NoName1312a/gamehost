package auth

import (
	"testing"
	"time"
)

func TestPasswordSetVerify(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if s.HasPassword() {
		t.Fatal("fresh store should have no password")
	}
	if s.Verify("anything") {
		t.Fatal("verify must fail when no password is set")
	}
	if err := s.SetPassword("hunter2"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	if !s.HasPassword() {
		t.Fatal("HasPassword should be true after SetPassword")
	}
	if !s.Verify("hunter2") {
		t.Error("correct password should verify")
	}
	if s.Verify("wrong") {
		t.Error("wrong password must not verify")
	}
}

func TestPasswordPersists(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := s.SetPassword("s3cret"); err != nil {
		t.Fatalf("set: %v", err)
	}
	// A fresh store over the same dir loads the persisted hash.
	s2, err := New(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if !s2.HasPassword() || !s2.Verify("s3cret") {
		t.Error("password did not persist across reopen")
	}
}

func TestSessions(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	tok := s.CreateSession(time.Hour)
	if tok == "" {
		t.Fatal("CreateSession returned empty token")
	}
	if !s.ValidateSession(tok) {
		t.Error("fresh session should validate")
	}
	if s.ValidateSession("not-a-real-token") {
		t.Error("bogus token must not validate")
	}
	s.DeleteSession(tok)
	if s.ValidateSession(tok) {
		t.Error("deleted session must not validate")
	}
}

func TestSessionExpiry(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	tok := s.CreateSession(-1 * time.Second) // already expired
	if s.ValidateSession(tok) {
		t.Error("expired session must not validate")
	}
}
