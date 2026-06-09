package basic

import "testing"

func TestHashAndVerifyRoundTrip(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("VerifyPassword returned false for the original password")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("VerifyPassword accepted a different password")
	}
}

func TestHashPasswordRejectsEmpty(t *testing.T) {
	t.Parallel()

	_, err := HashPassword("")
	if err == nil {
		t.Fatal("HashPassword(\"\") returned nil error")
	}
}

func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"not-a-hash",
		"pbkdf2-sha256$abc$salt$hash",
		"pbkdf2-sha256$1000$!!!invalid-base64!!!$hash",
		"pbkdf2-sha256$1000$dGVzdA$!!notbase64",
		"different-algo$1000$dGVzdA$dGVzdA",
	}
	for _, encoded := range cases {
		if VerifyPassword(encoded, "anything") {
			t.Errorf("VerifyPassword accepted malformed input %q", encoded)
		}
	}
}

func TestHashesAreSalted(t *testing.T) {
	t.Parallel()

	a, err := HashPassword("same-input")
	if err != nil {
		t.Fatalf("first HashPassword: %v", err)
	}
	b, err := HashPassword("same-input")
	if err != nil {
		t.Fatalf("second HashPassword: %v", err)
	}
	if a == b {
		t.Fatal("two hashes of the same password are identical — salt is missing")
	}
}
