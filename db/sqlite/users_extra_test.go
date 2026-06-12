package sqlite

import (
	"strings"
	"sync"
	"testing"

	"github.com/crgimenes/devengine/db"
)

func TestCreateUserRejectsDuplicateUsernameCaseInsensitive(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	_, err := s.CreateUser("Alice", "a@example.com", "h", false)
	if err != nil {
		t.Fatalf("CreateUser(Alice): %v", err)
	}

	_, err = s.CreateUser("alice", "b@example.com", "h", false)
	if err == nil {
		t.Fatal("CreateUser(alice) succeeded — case-insensitive uniqueness broken")
	}

	_, err = s.CreateUser("ALICE", "c@example.com", "h", false)
	if err == nil {
		t.Fatal("CreateUser(ALICE) succeeded — case-insensitive uniqueness broken")
	}
}

func TestCreateUserConcurrentUniqueness(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	const n = 20
	const username = "racer"

	var wg sync.WaitGroup
	results := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.CreateUser(username, "", "h", false)
			results[i] = err
		}(i)
	}
	wg.Wait()

	wins := 0
	for _, err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("got %d successes across %d concurrent CreateUser calls, want exactly 1", wins, n)
	}
}

func TestUpdateUserProfileRejectsEmptyUsername(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	u, _ := s.CreateUser("bob", "bob@example.com", "h", false)

	_, err := s.UpdateUserProfile(u.ID, "", "", "")
	if err == nil {
		t.Fatal("UpdateUserProfile with empty username succeeded")
	}
}

func TestUpdateUserProfileRejectsZeroUserID(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	_, err := s.UpdateUserProfile(0, "name", "", "")
	if err == nil {
		t.Fatal("UpdateUserProfile with userID=0 succeeded")
	}
}

func TestUpdateUserProfileRejectsNonExistentUser(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	_, err := s.UpdateUserProfile(9999, "name", "", "")
	if err == nil {
		t.Fatal("UpdateUserProfile on non-existent user succeeded")
	}
}

func TestUpdateUserProfileAvatarOnlyChange(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	u, _ := s.CreateUser("carol", "carol@example.com", "h", false)

	got, err := s.UpdateUserProfile(u.ID, "carol", "https://example.com/a.png", "carol@example.com")
	if err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}
	if got.Username != "carol" {
		t.Errorf("Username = %q, want carol", got.Username)
	}
	if got.AvatarURL != "https://example.com/a.png" {
		t.Errorf("AvatarURL = %q", got.AvatarURL)
	}
	if !got.Enabled {
		t.Error("Enabled = false after profile update")
	}
}

func TestUpdateUserProfileRejectsDuplicateUsername(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	a, _ := s.CreateUser("dave", "dave@example.com", "h", false)
	_, _ = s.CreateUser("eve", "eve@example.com", "h", false)

	_, err := s.UpdateUserProfile(a.ID, "EVE", "", "")
	if err == nil {
		t.Fatal("UpdateUserProfile to existing username (case-insensitive) succeeded")
	}
}

func TestUpdateUserProfileRejectsInvalidUsername(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	u, _ := s.CreateUser("frank", "frank@example.com", "h", false)

	cases := []string{
		"ab",
		strings.Repeat("a", 31),
		"has space",
		"hás-acento",
		"with/slash",
	}
	for _, name := range cases {
		_, err := s.UpdateUserProfile(u.ID, name, "", "")
		if err == nil {
			t.Errorf("UpdateUserProfile(%q) accepted invalid username", name)
		}
	}
}

func TestUpdateUserProfileChangesEmail(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	u, _ := s.CreateUser("grace", "grace@example.com", "h", false)

	got, err := s.UpdateUserProfile(u.ID, "grace", "", "newgrace@example.com")
	if err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}
	if got.Email != "newgrace@example.com" {
		t.Fatalf("Email = %q, want newgrace@example.com", got.Email)
	}
}

func TestUpdateUserProfileClearsEmail(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	u, _ := s.CreateUser("heidi", "heidi@example.com", "h", false)

	got, err := s.UpdateUserProfile(u.ID, "heidi", "", "")
	if err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}
	if got.Email != "" {
		t.Fatalf("Email = %q, want empty", got.Email)
	}
}

func TestUpdateUserProfileRejectsDuplicateEmail(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	a, _ := s.CreateUser("ivan", "ivan@example.com", "h", false)
	_, _ = s.CreateUser("judy", "judy@example.com", "h", false)

	_, err := s.UpdateUserProfile(a.ID, "ivan", "", "JUDY@example.com")
	if err == nil {
		t.Fatal("UpdateUserProfile to existing email (case-insensitive) succeeded")
	}
}

func TestUpdateUserProfileWithoutEmailDoesNotRequireOne(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	// db.User created with no email — the bug we're fixing rejected this on save.
	u, _ := s.CreateUser("ken", "", "h", false)

	got, err := s.UpdateUserProfile(u.ID, "ken-renamed", "", "")
	if err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}
	if got.Username != "ken-renamed" {
		t.Fatalf("Username = %q", got.Username)
	}
	if got.Email != "" {
		t.Fatalf("Email = %q, want empty", got.Email)
	}
	if !got.Enabled {
		t.Fatal("Enabled = false")
	}
}

func TestGenerateUniqueUsernameBaseFree(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	got, err := s.GenerateUniqueUsername("free")
	if err != nil {
		t.Fatalf("GenerateUniqueUsername: %v", err)
	}
	if got != "free" {
		t.Fatalf("got %q, want %q", got, "free")
	}
}

func TestGenerateUniqueUsernameMultipleConflicts(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	_, _ = s.CreateUser("user", "u1@example.com", "", false)
	_, _ = s.CreateUser("user1", "u2@example.com", "", false)
	_, _ = s.CreateUser("user2", "u3@example.com", "", false)
	_, _ = s.CreateUser("user3", "u4@example.com", "", false)

	got, err := s.GenerateUniqueUsername("user")
	if err != nil {
		t.Fatalf("GenerateUniqueUsername: %v", err)
	}
	if got != "user4" {
		t.Fatalf("got %q, want %q", got, "user4")
	}
}

func TestGenerateUniqueUsernameCaseInsensitive(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	_, _ = s.CreateUser("Alice", "a@example.com", "", false)

	got, err := s.GenerateUniqueUsername("alice")
	if err != nil {
		t.Fatalf("GenerateUniqueUsername: %v", err)
	}
	if got == "alice" {
		t.Fatalf("got %q, want a non-conflicting suffix", got)
	}
}

func TestIsValidUsername(t *testing.T) {
	t.Parallel()

	good := []string{"abc", "user_name", "user-name", "abc123", strings.Repeat("a", 30)}
	for _, s := range good {
		if err := db.IsValidUsername(s); err != nil {
			t.Errorf("db.IsValidUsername(%q) = %v, want nil", s, err)
		}
	}

	bad := []string{"", "ab", strings.Repeat("a", 31), "with space", "ola.mundo", "café", "a/b"}
	for _, s := range bad {
		if err := db.IsValidUsername(s); err == nil {
			t.Errorf("db.IsValidUsername(%q) accepted invalid username", s)
		}
	}
}
