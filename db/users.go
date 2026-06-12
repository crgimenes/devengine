package db

import (
	"errors"
)

// IsValidUsername validates a username to prevent abuse.
func IsValidUsername(username string) error {
	if len(username) < 3 || len(username) > 30 {
		return errors.New("username must be between 3 and 30 characters")
	}

	for _, r := range username {
		if (r < 'a' || r > 'z') &&
			(r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') &&
			r != '_' && r != '-' {
			return errors.New("username contains invalid characters")
		}
	}

	return nil
}
