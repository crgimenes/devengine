package utils

import (
	"errors"
	"strings"
	"testing"
)

func TestNewOpaqueID(t *testing.T) {
	a := NewOpaqueID()
	b := NewOpaqueID()
	if a == b {
		t.Fatal("opaque ids must be unique")
	}
	if !ValidateOpaqueID(a) || !ValidateOpaqueID(b) {
		t.Fatalf("generated id fails its own validation: %q", a)
	}

	short := NewOpaqueIDShort()
	if short == "" || len(short) >= len(a) {
		t.Fatalf("short id = %q (full id %q)", short, a)
	}
}

func TestValidateOpaqueID(t *testing.T) {
	if ValidateOpaqueID("") {
		t.Error("empty id accepted")
	}
	if ValidateOpaqueID("short") {
		t.Error("short id accepted")
	}
	if ValidateOpaqueID(strings.Repeat("!", 43)) {
		t.Error("invalid characters accepted")
	}
}

func TestMakePKCE(t *testing.T) {
	verifier, challenge := MakePKCE()
	if verifier == "" || challenge == "" || verifier == challenge {
		t.Fatalf("PKCE = %q, %q", verifier, challenge)
	}
}

func TestParseIntWithDefault(t *testing.T) {
	if got := ParseIntWithDefault("42", 7); got != 42 {
		t.Fatalf("valid = %d", got)
	}
	if got := ParseIntWithDefault("not a number", 7); got != 7 {
		t.Fatalf("invalid = %d, want default", got)
	}
	if got := ParseIntWithDefault("", 7); got != 7 {
		t.Fatalf("empty = %d, want default", got)
	}
}

func TestCanonicalizeEmail(t *testing.T) {
	got, err := CanonicalizeEmail("  Ana.Souza@Example.COM ")
	if err != nil {
		t.Fatalf("CanonicalizeEmail: %v", err)
	}
	if got != strings.ToLower(strings.TrimSpace(got)) {
		t.Fatalf("not canonical: %q", got)
	}

	for _, bad := range []string{"", "not-an-email", "a@", "@b"} {
		if _, err := CanonicalizeEmail(bad); err == nil {
			t.Errorf("CanonicalizeEmail(%q) accepted", bad)
		}
	}
}

func TestSanitizeDescription(t *testing.T) {
	if got := SanitizeDescription("  hello\x00world  "); strings.ContainsRune(got, 0) {
		t.Fatalf("control char survived: %q", got)
	}
	if got := SanitizeDescription(""); got != "" {
		t.Fatalf("empty = %q", got)
	}
}

func TestTagErrorMessage(t *testing.T) {
	err := newTagError("bad tag")
	if err.Error() != "bad tag" {
		t.Fatalf("Error() = %q", err.Error())
	}
	var te TagError
	if !errors.As(err, &te) {
		t.Fatal("errors.As failed")
	}
}

type failingCloser struct{}

func (failingCloser) Close() error { return errors.New("boom") }

func TestCloserSwallowsErrors(t *testing.T) {
	// Must not panic on error nor on nil.
	Closer(failingCloser{})
	Closer(nil)
}
