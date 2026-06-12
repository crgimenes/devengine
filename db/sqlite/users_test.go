package sqlite

import "testing"

func TestCreateUserAndLookup(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	const username = "alice"
	const email = "alice@example.com"
	const hash = "pbkdf2-sha256$1$abc$def"

	u, err := s.CreateUser(username, email, hash, true)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Username != username {
		t.Fatalf("Username = %q, want %q", u.Username, username)
	}
	if u.Email != email {
		t.Fatalf("Email = %q, want %q", u.Email, email)
	}
	if u.PasswordHash != hash {
		t.Fatalf("PasswordHash = %q, want %q", u.PasswordHash, hash)
	}
	if !u.Sysop {
		t.Fatal("Sysop = false, want true")
	}
	if !u.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if u.ReferenceID == "" {
		t.Fatal("ReferenceID is empty — trigger did not fire?")
	}

	byID, err := s.GetUserByID(u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if byID.ID != u.ID {
		t.Fatalf("GetUserByID ID mismatch")
	}

	byUser, err := s.GetUserByUsername(username)
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if byUser == nil || byUser.ID != u.ID {
		t.Fatalf("GetUserByUsername did not find user")
	}

	missing, err := s.GetUserByUsername("noone")
	if err != nil {
		t.Fatalf("GetUserByUsername(missing): %v", err)
	}
	if missing != nil {
		t.Fatalf("GetUserByUsername(missing) = %+v, want nil", missing)
	}
}

func TestCountUsers(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	n, err := s.CountUsers()
	if err != nil {
		t.Fatalf("CountUsers (empty): %v", err)
	}
	if n != 0 {
		t.Fatalf("CountUsers (empty) = %d, want 0", n)
	}

	_, err = s.CreateUser("bob", "bob@example.com", "h", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	n, err = s.CountUsers()
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	if n != 1 {
		t.Fatalf("CountUsers = %d, want 1", n)
	}
}

func TestSetPasswordHash(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	u, err := s.CreateUser("carol", "carol@example.com", "old", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	err = s.SetPasswordHash(u.ID, "new")
	if err != nil {
		t.Fatalf("SetPasswordHash: %v", err)
	}

	reloaded, err := s.GetUserByID(u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if reloaded.PasswordHash != "new" {
		t.Fatalf("PasswordHash = %q, want %q", reloaded.PasswordHash, "new")
	}
}
