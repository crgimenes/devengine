package utils

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode"
)

// CanonicalizeEmail returns (canonical, error).
// Policy: case-insensitive matching globally -> lower(local + domain).
// Strips Unicode control-format characters (e.g., zero-width) and trailing dot
// in the domain. RFC 5322 syntax must parse successfully.
func CanonicalizeEmail(input string) (string, error) {
	if len(input) > 254 {
		return "", fmt.Errorf("email too long")
	}

	s := strings.TrimSpace(input)

	addr, err := mail.ParseAddress(s)
	if err != nil {
		return "", fmt.Errorf("invalid email syntax: %w", err)
	}
	original := addr.Address

	var b strings.Builder
	for _, r := range original {
		if unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
	}
	clean := b.String()

	at := strings.LastIndexByte(clean, '@')
	if at <= 0 || at == len(clean)-1 {
		return "", fmt.Errorf("invalid email address")
	}
	local := clean[:at]
	domain := clean[at+1:]

	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	local = strings.ToLower(local)

	if domain == "" || local == "" {
		return "", fmt.Errorf("empty local or domain")
	}

	return local + "@" + domain, nil
}
