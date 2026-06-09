package auth

import (
	"os"
	"path/filepath"
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

func TestMultiUserAddVerifyDelete(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := s.SetPassword("owner-pass-1"); err != nil {
		t.Fatalf("set owner: %v", err)
	}
	if err := s.AddUser("alice", "alicepw12", RoleOperator); err != nil {
		t.Fatalf("add user: %v", err)
	}
	if !s.VerifyUser("alice", "alicepw12") || s.VerifyUser("alice", "wrong") {
		t.Error("alice verification wrong")
	}
	if role, ok := s.UserRole("alice"); !ok || role != RoleOperator {
		t.Errorf("alice role = %q", role)
	}
	// Reserved name, weak password, bad role are rejected.
	if err := s.AddUser("owner", "anything8", RoleOperator); err == nil {
		t.Error("owner username must be reserved")
	}
	if err := s.AddUser("bob", "short", RoleOperator); err == nil {
		t.Error("short password must be rejected")
	}
	if err := s.AddUser("bob", "bobpw1234", "superuser"); err == nil {
		t.Error("invalid role must be rejected")
	}
	if len(s.ListUsers()) != 2 {
		t.Errorf("expected owner + alice, got %+v", s.ListUsers())
	}
	// Owner can't be deleted; alice can.
	if err := s.DeleteUser("owner"); err == nil {
		t.Error("owner must not be deletable")
	}
	if err := s.DeleteUser("alice"); err != nil {
		t.Fatalf("delete alice: %v", err)
	}
	if s.VerifyUser("alice", "alicepw12") {
		t.Error("deleted user must not verify")
	}
}

func TestLegacyCredentialMigratesToOwner(t *testing.T) {
	dir := t.TempDir()
	legacy := `{"algo":"argon2id","salt":"c2FsdHNhbHRzYWx0","hash":"aGFzaGhhc2hoYXNo","time":1,"memory":65536,"threads":4}`
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := New(dir)
	if err != nil {
		t.Fatalf("new over legacy: %v", err)
	}
	if !s.HasPassword() {
		t.Error("legacy credential should migrate to a set password")
	}
	users := s.ListUsers()
	if len(users) != 1 || users[0].Username != "owner" || users[0].Role != RoleOwner {
		t.Errorf("legacy did not migrate to a single owner: %+v", users)
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
