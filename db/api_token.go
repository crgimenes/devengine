package db

import "time"

// APIToken is a long-lived bearer credential for the REST API. Only the
// SHA-256 hash of the token is stored; the plaintext is shown to the user
// once at creation and never persisted.
type APIToken struct {
	ID        int64
	UserID    int64
	Label     string
	CreatedAt time.Time
}
