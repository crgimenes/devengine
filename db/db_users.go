package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/crgimenes/devengine/mail"
)

// GetUserByID retrieves a user by their ID.
func (s *SQLite) GetUserByID(userID int64) (*User, error) {
	const sqlSelect = `SELECT
            id,                         -- 1
            reference_id,               -- 2
            COALESCE(username, ''),     -- 3
            email,                      -- 4
            COALESCE(avatar_url, ''),   -- 5
            enabled,                    -- 6
            sysop                       -- 7
        FROM users
        WHERE id = ?  -- 1
        LIMIT 1;`

	var u User

	err := s.QueryRow(
		sqlSelect,
		userID, // 1
	).Scan(
		&u.ID,          // 1
		&u.ReferenceID, // 2
		&u.Username,    // 3
		&u.Email,       // 4
		&u.AvatarURL,   // 5
		&u.Enabled,     // 6
		&u.Sysop,       // 7
	)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// GetUserByEmail retrieves a user by their email address.
// Returns error if user not found.
func (s *SQLite) GetUserByEmail(email string) (*User, error) {
	const sqlSelect = `SELECT
            id,                         -- 1
            reference_id,               -- 2
            COALESCE(username, ''),     -- 3
            email,                      -- 4
            COALESCE(avatar_url, ''),   -- 5
            enabled,                    -- 6
            sysop                       -- 7
        FROM users
        WHERE email = ?  -- 1
        LIMIT 1;`

	email, err := mail.CanonicalizeEmail(email)
	if err != nil {
		return nil, err
	}

	var u User
	err = s.QueryRow(
		sqlSelect,
		email, // 1
	).Scan(
		&u.ID,          // 1
		&u.ReferenceID, // 2
		&u.Username,    // 3
		&u.Email,       // 4
		&u.AvatarURL,   // 5
		&u.Enabled,     // 6
		&u.Sysop,       // 7
	)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// GetUserOrCreateByEmail creates or retrieves a user by email.
func (s *SQLite) GetUserOrCreateByEmail(email string) (*User, error) {
	const sqlSelect = `SELECT
            id,                         -- 1
            reference_id,               -- 2
            COALESCE(username, ''),     -- 3
            email,                      -- 4
            COALESCE(avatar_url, ''),   -- 5
            enabled,                    -- 6
            sysop                       -- 7
        FROM users
        WHERE email = ?  -- 1
        LIMIT 1;`

	var u User

	email, err := mail.CanonicalizeEmail(email)
	if err != nil {
		return nil, err
	}

	err = s.QueryRow(
		sqlSelect,
		email, // 1
	).Scan(
		&u.ID,          // 1
		&u.ReferenceID, // 2
		&u.Username,    // 3
		&u.Email,       // 4
		&u.AvatarURL,   // 5
		&u.Enabled,     // 6
		&u.Sysop,       // 7
	)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if err == nil {
		// User already exists
		return &u, nil
	}

	// Check if this is the first user
	const sqlCountUsers = `SELECT COUNT(*) FROM users`
	var userCount int
	countErr := s.QueryRow(sqlCountUsers).Scan(&userCount)
	if countErr != nil {
		return nil, countErr
	}

	// First user gets sysop = 1, subsequent users get sysop = 0
	sysopValue := 0
	if userCount == 0 {
		sysopValue = 1
	}

	const sqlInsert = `INSERT INTO users (
            email,             -- 1
            sysop,             -- 2
            created_at,
            updated_at
        ) VALUES (
            ?,                 -- 1
            ?,                 -- 2
            CURRENT_TIMESTAMP, -- created_at
            CURRENT_TIMESTAMP  -- updated_at
        )
        RETURNING
            id,                        -- 1
            reference_id,              -- 2
            COALESCE(username, ''),    -- 3
            email,                     -- 4
            COALESCE(avatar_url, ''),  -- 5
            enabled,                   -- 6
            sysop;` // 7

	err = s.QueryRowRW(
		sqlInsert,
		email,      // 1
		sysopValue, // 2
	).Scan(
		&u.ID,          // 1
		&u.ReferenceID, // 2
		&u.Username,    // 3
		&u.Email,       // 4
		&u.AvatarURL,   // 5
		&u.Enabled,     // 6
		&u.Sysop,       // 7
	)
	if err != nil {
		return nil, err
	}

	// IMPORTANT: ReferenceID is filled by an AFTER INSERT trigger.
	// When using RETURNING, SQLite returns values prior to AFTER triggers.
	// Reload user to ensure ReferenceID (and other trigger-updated fields) are populated.
	fresh, err := s.GetUserByID(u.ID)
	if err != nil {
		return nil, err
	}
	return fresh, nil
}

// CountUsersWithUsernamePrefix counts how many users have a username starting with the given prefix (case-insensitive).
// Used for generating unique usernames when conflicts occur.
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

// GenerateUniqueUsername generates a unique username by appending a number if the base username is taken.
// If baseUsername is available, returns it unchanged.
// Otherwise, appends 1, 2, 3, etc. until a unique username is found.
func (s *SQLite) GenerateUniqueUsername(baseUsername string) (string, error) {
	const sqlCheck = `SELECT COUNT(*)
        FROM users
        WHERE LOWER(username) = LOWER(?)`

	// Check if base username is available
	var count int
	err := s.QueryRow(sqlCheck, baseUsername).Scan(&count)
	if err != nil {
		return "", err
	}

	if count == 0 {
		return baseUsername, nil // Base username is available
	}

	// Base username is taken, find how many variants exist with this prefix
	existingCount, err := s.CountUsersWithUsernamePrefix(baseUsername)
	if err != nil {
		return "", err
	}

	// Try appending numbers until we find an available username
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

// UpdateUserProfile updates username, avatar_url and enables user if email is
// already validated (email must be non-empty). Validates that username and
// email are not duplicated (case-insensitive). Returns the updated user or
// error if validation fails or SQL error occurs.
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

	// Get current user to verify email is present
	currentUser, err := s.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	if currentUser.Email == "" {
		return nil, errors.New("user has no email; cannot enable account")
	}

	// Check username uniqueness (case-insensitive)
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

	// Update user: set username, avatar_url, and enable
	const sqlUpdate = `UPDATE users
        SET
            username = ?,    -- 1
            avatar_url = ?,  -- 2
            enabled = 1
        WHERE id = ?         -- 3
        RETURNING
            id,                        -- 1
            reference_id,              -- 2
            COALESCE(username, ''),    -- 3
            email,                     -- 4
            COALESCE(avatar_url, ''),  -- 5
            enabled,                   -- 6
            sysop;` // 7

	var u User
	err = s.QueryRowRW(
		sqlUpdate,
		username,  // 1
		avatarURL, // 2
		userID,    // 3
	).Scan(
		&u.ID,          // 1
		&u.ReferenceID, // 2
		&u.Username,    // 3
		&u.Email,       // 4
		&u.AvatarURL,   // 5
		&u.Enabled,     // 6
		&u.Sysop,       // 7
	)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// MergeOAuthProfileData merges OAuth provider data into existing user,
// updating only blank fields.
// It updates username (with conflict resolution) if Username is
// empty and oauthUsername is provided.
// It updates avatar_url if user.AvatarURL is empty and oauthAvatarURL is provided.
// Returns updated user with enabled=true if both username and email
// are now present, or error.
func (s *SQLite) MergeOAuthProfileData(
	userID int64,
	oauthUsername string,
	oauthAvatarURL string,
) (*User, error) {

	if userID == 0 {
		return nil, errors.New("user_id is required")
	}

	// Validate username from OAuth provider - if invalid, treat as empty
	if oauthUsername != "" {
		if err := IsValidUsername(oauthUsername); err != nil {
			oauthUsername = "" // Treat invalid username as empty
		}
	}

	// Get current user
	currentUser, err := s.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	if currentUser.Email == "" {
		return nil, errors.New("user has no email; cannot update")
	}

	// Determine which fields need updating
	newUsername := currentUser.Username
	newAvatarURL := currentUser.AvatarURL

	// Update username only if current is empty and OAuth provides one
	if oauthUsername != "" && currentUser.Username == "" {
		uniqueUsername, err := s.GenerateUniqueUsername(oauthUsername)
		if err != nil {
			return nil, err
		}
		newUsername = uniqueUsername
	}

	// Update avatar_url only if current is empty and OAuth provides one
	if oauthAvatarURL != "" && currentUser.AvatarURL == "" {
		newAvatarURL = oauthAvatarURL
	}

	// If nothing changed, return current user as-is
	if newUsername == currentUser.Username && newAvatarURL == currentUser.AvatarURL {
		return currentUser, nil
	}

	// Determine if user should be enabled (both username and email must be present)
	shouldBeEnabled := newUsername != "" && currentUser.Email != ""

	// Update user with new values
	const sqlUpdate = `UPDATE users
        SET
            username = ?,    -- 1
            avatar_url = ?,  -- 2
            enabled = ?      -- 3
        WHERE id = ?         -- 4
        RETURNING
            id,                       -- 1
            reference_id,             -- 2
            COALESCE(username, ''),   -- 3
            email,                    -- 4
            COALESCE(avatar_url, ''), -- 5
            enabled,                  -- 6
            sysop;` // 7

	var u User
	enabled := 0
	if shouldBeEnabled {
		enabled = 1
	}

	err = s.QueryRowRW(
		sqlUpdate,
		newUsername,  // 1
		newAvatarURL, // 2
		enabled,      // 3
		userID,       // 4
	).Scan(
		&u.ID,          // 1
		&u.ReferenceID, // 2
		&u.Username,    // 3
		&u.Email,       // 4
		&u.AvatarURL,   // 5
		&u.Enabled,     // 6
		&u.Sysop,       // 7
	)
	if err != nil {
		return nil, err
	}

	return &u, nil
}
