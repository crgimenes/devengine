package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/crgimenes/devengine/utils"
)

const userSelectColumns = `id,                         -- 1
            reference_id,               -- 2
            COALESCE(username, ''),     -- 3
            COALESCE(email, ''),        -- 4
            COALESCE(password_hash, ''),-- 5
            COALESCE(avatar_url, ''),   -- 6
            enabled,                    -- 7
            sysop                       -- 8`

func scanUser(scanner interface {
	Scan(dest ...any) error
}) (*User, error) {
	var u User
	err := scanner.Scan(
		&u.ID,           // 1
		&u.ReferenceID,  // 2
		&u.Username,     // 3
		&u.Email,        // 4
		&u.PasswordHash, // 5
		&u.AvatarURL,    // 6
		&u.Enabled,      // 7
		&u.Sysop,        // 8
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByID retrieves a user by their ID.
func (s *SQLite) GetUserByID(userID int64) (*User, error) {
	sqlSelect := `SELECT ` + userSelectColumns + `
        FROM users
        WHERE id = ?  -- 1
        LIMIT 1;`

	return scanUser(s.QueryRow(
		sqlSelect,
		userID, // 1
	))
}

// GetUserByEmail retrieves a user by their email address.
func (s *SQLite) GetUserByEmail(email string) (*User, error) {
	sqlSelect := `SELECT ` + userSelectColumns + `
        FROM users
        WHERE email = ?  -- 1
        LIMIT 1;`

	email, err := utils.CanonicalizeEmail(email)
	if err != nil {
		return nil, err
	}

	return scanUser(s.QueryRow(
		sqlSelect,
		email, // 1
	))
}

// GetUserByUsername retrieves a user by their (case-insensitive) username.
// Returns nil if no user matches.
func (s *SQLite) GetUserByUsername(username string) (*User, error) {
	sqlSelect := `SELECT ` + userSelectColumns + `
        FROM users
        WHERE LOWER(username) = LOWER(?)  -- 1
        LIMIT 1;`

	u, err := scanUser(s.QueryRow(
		sqlSelect,
		username, // 1
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

// CountUsers returns the total number of users in the database. Useful for
// detecting first-run conditions during bootstrap.
func (s *SQLite) CountUsers() (int, error) {
	var n int
	err := s.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// CreateUser inserts a new user row. Caller is responsible for hashing the
// password before calling. Used by the bootstrap CLI (sysop admin) and by the
// invite signup flow (regular user). Returns the row reloaded from disk so the
// reference_id trigger has fired.
func (s *SQLite) CreateUser(
	username string,
	email string,
	passwordHash string,
	sysop bool,
) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)

	if username == "" {
		return nil, errors.New("username is required")
	}
	err := IsValidUsername(username)
	if err != nil {
		return nil, err
	}

	if email != "" {
		email, err = utils.CanonicalizeEmail(email)
		if err != nil {
			return nil, err
		}
	}

	const sqlInsert = `INSERT INTO users (
            username,          -- 1
            email,             -- 2
            password_hash,     -- 3
            sysop,             -- 4
            enabled,           -- 5
            created_at,
            updated_at
        ) VALUES (
            ?,                 -- 1
            NULLIF(?, ''),     -- 2
            NULLIF(?, ''),     -- 3
            ?,                 -- 4
            1,                 -- 5
            CURRENT_TIMESTAMP,
            CURRENT_TIMESTAMP
        )
        RETURNING id;`

	sysopValue := 0
	if sysop {
		sysopValue = 1
	}

	var id int64
	err = s.QueryRowRW(
		sqlInsert,
		username,     // 1
		email,        // 2
		passwordHash, // 3
		sysopValue,   // 4
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	return s.GetUserByID(id)
}

// SetPasswordHash updates the password hash for an existing user.
func (s *SQLite) SetPasswordHash(userID int64, passwordHash string) error {
	const sqlUpdate = `UPDATE users
        SET password_hash = NULLIF(?, '')  -- 1
        WHERE id = ?                       -- 2`

	return s.Exec(
		sqlUpdate,
		passwordHash, // 1
		userID,       // 2
	)
}

// CountUsersWithUsernamePrefix counts how many users have a username starting
// with the given prefix (case-insensitive). Useful for generating unique
// usernames when conflicts occur.
func (s *SQLite) CountUsersWithUsernamePrefix(prefix string) (int, error) {
	const sqlStatement = `SELECT COUNT(*)
        FROM users
        WHERE LOWER(username) LIKE LOWER(?) || '%'`

	var count int
	err := s.QueryRow(sqlStatement, prefix).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GenerateUniqueUsername returns the base username when available, otherwise
// appends a numeric suffix until a free name is found.
func (s *SQLite) GenerateUniqueUsername(baseUsername string) (string, error) {
	const sqlCheck = `SELECT COUNT(*)
        FROM users
        WHERE LOWER(username) = LOWER(?)`

	var count int
	err := s.QueryRow(sqlCheck, baseUsername).Scan(&count)
	if err != nil {
		return "", err
	}

	if count == 0 {
		return baseUsername, nil
	}

	existingCount, err := s.CountUsersWithUsernamePrefix(baseUsername)
	if err != nil {
		return "", err
	}

	for i := 1; i <= existingCount+10; i++ {
		candidate := baseUsername + fmt.Sprintf("%d", i)
		err := s.QueryRow(sqlCheck, candidate).Scan(&count)
		if err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not generate unique username for %s", baseUsername)
}

// IsValidUsername validates a username to prevent abuse.
func IsValidUsername(username string) error {
	if len(username) < 3 || len(username) > 30 {
		return errors.New("username must be between 3 and 30 characters")
	}

	for _, r := range username {
		if !(r >= 'a' && r <= 'z') &&
			!(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') &&
			r != '_' && r != '-' {
			return errors.New("username contains invalid characters")
		}
	}

	return nil
}

// UpdateUserProfile updates username and avatar_url; enables the user once
// both username and email are present. Validates uniqueness of username
// case-insensitively.
func (s *SQLite) UpdateUserProfile(
	userID int64,
	username string,
	avatarURL string,
) (*User, error) {

	if userID == 0 {
		return nil, errors.New("user_id is required")
	}

	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username is required")
	}

	err := IsValidUsername(username)
	if err != nil {
		return nil, err
	}

	avatarURL = strings.TrimSpace(avatarURL)

	currentUser, err := s.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	if currentUser.Email == "" {
		return nil, errors.New("user has no email; cannot enable account")
	}

	const sqlCheckUsername = `SELECT COUNT(*)
        FROM users
        WHERE LOWER(username) = LOWER(?)
        AND id != ?
        LIMIT 1;`

	var count int
	err = s.QueryRow(sqlCheckUsername, username, userID).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("username is already in use")
	}

	const sqlUpdate = `UPDATE users
        SET
            username = ?,    -- 1
            avatar_url = ?,  -- 2
            enabled = 1
        WHERE id = ?         -- 3`

	err = s.Exec(
		sqlUpdate,
		username,  // 1
		avatarURL, // 2
		userID,    // 3
	)
	if err != nil {
		return nil, err
	}

	return s.GetUserByID(userID)
}
