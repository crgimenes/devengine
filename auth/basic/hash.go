package basic

// Password hashing using PBKDF2-HMAC-SHA256.
//
// PBKDF2 is the strongest password hash available in the Go standard library
// today (Go 1.24+ promoted crypto/pbkdf2). Argon2id would be preferable, but
// devengine sticks to the standard library; switching can happen when stdlib
// catches up. The encoded format is forward-compatible:
//
//   pbkdf2-sha256$<iterations>$<salt-b64>$<hash-b64>
//
// where salt and hash are RawStdEncoding (base64 without padding).

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	pbkdf2Iterations = 600_000
	pbkdf2KeyLen     = 32
	pbkdf2SaltLen    = 16
	pbkdf2Prefix     = "pbkdf2-sha256"
)

// HashPassword returns an encoded PBKDF2-SHA256 hash for the given plaintext.
func HashPassword(plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("password is required")
	}

	salt := make([]byte, pbkdf2SaltLen)
	_, err := rand.Read(salt)
	if err != nil {
		return "", fmt.Errorf("rand salt: %w", err)
	}

	dk, err := pbkdf2.Key(sha256.New, plaintext, salt, pbkdf2Iterations, pbkdf2KeyLen)
	if err != nil {
		return "", fmt.Errorf("pbkdf2: %w", err)
	}

	return fmt.Sprintf("%s$%d$%s$%s",
		pbkdf2Prefix,
		pbkdf2Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk),
	), nil
}

// VerifyPassword returns true when the plaintext matches the encoded hash.
// Comparison is constant-time to avoid leaking timing information.
func VerifyPassword(encoded, plaintext string) bool {
	if encoded == "" || plaintext == "" {
		return false
	}

	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return false
	}
	if parts[0] != pbkdf2Prefix {
		return false
	}

	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}

	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}

	got, err := pbkdf2.Key(sha256.New, plaintext, salt, iter, len(want))
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(want, got) == 1
}
