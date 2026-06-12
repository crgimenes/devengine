package sqlite

import (
	"testing"
	"time"
)

func TestStoreAndConsumeToken(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	const (
		token  = "tok-abc"
		email  = "user@example.com"
		action = "invite"
	)
	exp := time.Now().UTC().Add(1 * time.Hour)

	err := s.StoreToken(token, email, action, exp)
	if err != nil {
		t.Fatalf("StoreToken: %v", err)
	}

	got, err := s.ConsumeToken(token, action)
	if err != nil {
		t.Fatalf("ConsumeToken: %v", err)
	}
	if got != email {
		t.Fatalf("ConsumeToken email = %q, want %q", got, email)
	}

	// Second consumption must return empty (token removed).
	again, err := s.ConsumeToken(token, action)
	if err != nil {
		t.Fatalf("ConsumeToken (second): %v", err)
	}
	if again != "" {
		t.Fatalf("ConsumeToken (second) = %q, want \"\"", again)
	}
}

func TestConsumeTokenWrongAction(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	exp := time.Now().UTC().Add(1 * time.Hour)
	err := s.StoreToken("tok", "u@e.com", "invite", exp)
	if err != nil {
		t.Fatalf("StoreToken: %v", err)
	}

	// Try to consume with the wrong action — must not succeed.
	got, err := s.ConsumeToken("tok", "magic_link")
	if err != nil {
		t.Fatalf("ConsumeToken: %v", err)
	}
	if got != "" {
		t.Fatalf("ConsumeToken with wrong action returned %q, want \"\"", got)
	}

	// Original token must still be consumable with the right action.
	got, err = s.ConsumeToken("tok", "invite")
	if err != nil {
		t.Fatalf("ConsumeToken (right action): %v", err)
	}
	if got != "u@e.com" {
		t.Fatalf("ConsumeToken email = %q, want %q", got, "u@e.com")
	}
}

func TestPurgeExpiredTokens(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	past := time.Now().UTC().Add(-1 * time.Hour)
	future := time.Now().UTC().Add(1 * time.Hour)

	err := s.StoreToken("expired", "a@e.com", "invite", past)
	if err != nil {
		t.Fatalf("StoreToken (expired): %v", err)
	}
	err = s.StoreToken("alive", "b@e.com", "invite", future)
	if err != nil {
		t.Fatalf("StoreToken (alive): %v", err)
	}

	err = s.PurgeExpiredTokens()
	if err != nil {
		t.Fatalf("PurgeExpiredTokens: %v", err)
	}

	got, _ := s.ConsumeToken("expired", "invite")
	if got != "" {
		t.Fatalf("expired token survived purge: %q", got)
	}
	got, _ = s.ConsumeToken("alive", "invite")
	if got != "b@e.com" {
		t.Fatalf("live token = %q, want %q", got, "b@e.com")
	}
}
