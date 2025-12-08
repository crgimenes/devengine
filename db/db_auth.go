package db

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/crgimenes/devengine/mail"
)

// GetUserByOAuthProviderID retrieves a user ID by OAuth provider and provider ID.
func (s *SQLite) GetUserByOAuthProviderID(provider string, providerID string) (int64, error) {
	const sqlStatement = `SELECT
            u.id
        FROM users u
        JOIN identities i ON u.id = i.user_id
        WHERE i.provider = ?    -- 1
        AND i.provider_uid = ?  -- 2
        LIMIT 1;`

	row := s.QueryRow(
		sqlStatement,
		provider,   // 1
		providerID, // 2
	)
	var userID int64
	err := row.Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil // No user found
		}
		return 0, err
	}
	return userID, nil
}

// StoreMagicLinkToken stores a magic link token for email-based authentication.
func (s *SQLite) StoreMagicLinkToken(
	token string,
	email string,
	expiresAt time.Time) error {
	const sqlStatement = `INSERT INTO magic_token (
            email,         -- 1
            token,         -- 2
            expires_at,    -- 3
            action
        ) VALUES (
            ?,        -- 1
            ?,        -- 2
            ?,        -- 3
            'login'   -- action
        );`

	return s.Exec(
		sqlStatement,
		email,     // 1
		token,     // 2
		expiresAt, // 3
	)
}

// ConsumeMagicLinkToken validates and consumes a magic link token, returning the associated email.
func (s *SQLite) ConsumeMagicLinkToken(token string) (string, error) {
	const sqlSelect = `SELECT
            email
        FROM magic_token
        WHERE token = ?          -- 1
        AND expires_at > CURRENT_TIMESTAMP
        LIMIT 1;`

	var email string
	err := s.QueryRow(
		sqlSelect,
		token, // 1
	).Scan(&email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil // No token found
		}
		return "", err
	}

	const sqlDelete = `DELETE FROM magic_token
        WHERE token = ?;` // 1

	err = s.Exec(
		sqlDelete,
		token, // 1
	)
	if err != nil {
		return "", err
	}

	return email, nil
}

// PurgeExpiredMagicLinkTokens removes expired magic link tokens from the database.
func (s *SQLite) PurgeExpiredMagicLinkTokens() error {
	const sqlStatement = `DELETE FROM magic_token
        WHERE expires_at <= CURRENT_TIMESTAMP;`

	return s.Exec(sqlStatement)
}

// GetUserOrCreateByOAuth creates or retrieves user via OAuth provider.
func (s *SQLite) GetUserOrCreateByOAuth(
	provider string,
	providerID string,
	email string,
	username string,
	avatarURL string,
) (*User, error) {

	// Validate username from OAuth provider - if invalid, treat as empty
	if username != "" {
		if err := IsValidUsername(username); err != nil {
			username = "" // Treat invalid username as empty, user will be redirected to /me
		}
	}

	// First, check if we have an existing OAuth identity for this provider/providerID
	userID, err := s.GetUserByOAuthProviderID(provider, providerID)
	if err != nil {
		return nil, err
	}
	if userID != 0 {
		// User already has this OAuth identity
		// Still merge profile data in case OAuth provider now provides data it didn't before
		existingUser, err := s.MergeOAuthProfileData(userID, username, avatarURL)
		if err != nil {
			return nil, err
		}
		return existingUser, nil
	}

	// Check if a user with this email already exists (from magic link or another OAuth provider)
	if email != "" {
		emailUser, err := s.GetUserOrCreateByEmail(email)
		if err != nil {
			return nil, err
		}
		if emailUser != nil && emailUser.ID != 0 {
			// Email already exists, link this OAuth identity to the existing user
			const sqlInsertIdentity = `INSERT INTO identities (
                user_id,          -- 1
                provider,         -- 2
                provider_uid,     -- 3
                created_at,
                updated_at
            ) VALUES (
                ?,                 -- 1
                ?,                 -- 2
                ?,                 -- 3
                CURRENT_TIMESTAMP, -- created_at
                CURRENT_TIMESTAMP  -- updated_at
            );`

			err = s.Exec(
				sqlInsertIdentity,
				emailUser.ID, // 1
				provider,     // 2
				providerID,   // 3
			)
			if err != nil {
				return nil, err
			}

			// Update missing fields from OAuth data if they are empty in database
			emailUser, err = s.MergeOAuthProfileData(emailUser.ID, username, avatarURL)
			if err != nil {
				return nil, err
			}

			return emailUser, nil
		}
	}

	// No existing email or OAuth identity, create new user with conflict resolution for username
	actualUsername := username
	if username != "" {
		var err error
		actualUsername, err = s.GenerateUniqueUsername(username)
		if err != nil {
			return nil, err
		}
	}

	u := User{
		Username:  actualUsername,
		Email:     email,
		AvatarURL: avatarURL,
		Enabled:   false,
	}

	const sqlInsert = `INSERT INTO users (
            email,             -- 1
            username,          -- 2
            avatar_url,        -- 3
            enabled,           -- 4
            created_at,
            updated_at
        ) VALUES (
            ?,                 -- 1
            ?,                 -- 2
            ?,                 -- 3
            ?,                 -- 4
            CURRENT_TIMESTAMP, -- created_at
            CURRENT_TIMESTAMP  -- updated_at
        )
        RETURNING
            id,                         -- 1
            reference_id,               -- 2
            COALESCE(username, ''),     -- 3
            email,                      -- 4
            COALESCE(avatar_url, ''),   -- 5
            enabled;` // 6

	// enabled is true only if BOTH username AND email are present
	// (we assume email is validated by OAuth provider if present)
	enabled := email != "" && actualUsername != ""

	err = s.QueryRowRW(
		sqlInsert,
		email,          // 1
		actualUsername, // 2
		avatarURL,      // 3
		enabled,        // 4
	).Scan(
		&u.ID,          // 1
		&u.ReferenceID, // 2
		&u.Username,    // 3
		&u.Email,       // 4
		&u.AvatarURL,   // 5
		&u.Enabled,     // 6
	)
	if err != nil {
		return nil, err
	}

	// Reload to get trigger-populated fields (reference_id)
	uFresh, err := s.GetUserByID(u.ID)
	if err != nil {
		return nil, err
	}

	//-----------------------------------------
	const sqlInsertIdentity = `INSERT INTO identities (
            user_id,          -- 1
            provider,         -- 2
            provider_uid,     -- 3
            created_at,
            updated_at
        ) VALUES (
            ?,                 -- 1
            ?,                 -- 2
            ?,                 -- 3
            CURRENT_TIMESTAMP, -- created_at
            CURRENT_TIMESTAMP  -- updated_at
        );`

	// Use the fresh user's ID for identity linking
	err = s.Exec(
		sqlInsertIdentity,
		uFresh.ID,  // 1
		provider,   // 2
		providerID, // 3
	)
	if err != nil {
		return nil, err
	}

	return uFresh, nil
}

// CreateMinimalUserForOAuthFallback creates a minimal user account with email only
// when OAuth signup fails. This allows the user to complete their profile at /me.
// Returns error if email is empty or if database operation fails.
func (s *SQLite) CreateMinimalUserForOAuthFallback(
	email string,
	avatarURL string,
) (*User, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, errors.New("email is required for fallback user creation")
	}

	email, err := mail.CanonicalizeEmail(email)
	if err != nil {
		return nil, err
	}

	avatarURL = strings.TrimSpace(avatarURL)

	u := User{
		Username:  "", // User must complete this at /me
		Email:     email,
		AvatarURL: avatarURL,
		Enabled:   false, // Not enabled until username is set
	}

	const sqlInsert = `INSERT INTO users (
            email,             -- 1
            username,          -- 2
            avatar_url,        -- 3
            enabled,           -- 4
            created_at,
            updated_at
        ) VALUES (
            ?,                 -- 1
            ?,                 -- 2
            ?,                 -- 3
            ?,                 -- 4
            CURRENT_TIMESTAMP, -- created_at
            CURRENT_TIMESTAMP  -- updated_at
        )
        RETURNING
            id,                         -- 1
            reference_id,               -- 2
            COALESCE(username, ''),     -- 3
            email,                      -- 4
            COALESCE(avatar_url, ''),  -- 5
            enabled;` // 6

	err = s.QueryRowRW(
		sqlInsert,
		email,     // 1
		nil,       // 2 - NULL username (will be set at /me)
		avatarURL, // 3
		false,     // 4 - not enabled
	).Scan(
		&u.ID,          // 1
		&u.ReferenceID, // 2
		&u.Username,    // 3
		&u.Email,       // 4
		&u.AvatarURL,   // 5
		&u.Enabled,     // 6
	)
	if err != nil {
		return nil, err
	}

	// Reload to get trigger-populated reference_id
	uf, err := s.GetUserByID(u.ID)
	if err != nil {
		return nil, err
	}

	return uf, nil
}
