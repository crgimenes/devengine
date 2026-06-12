package sqlite

import (
	"database/sql"
	"errors"
	"time"
)

// StoreToken stores a single-use token in the magic_token table. The action
// string scopes the token (e.g. "magic_link", "invite", "password_reset") so a
// token issued for one flow cannot be consumed by another.
func (s *SQLite) StoreToken(
	token string,
	email string,
	action string,
	expiresAt time.Time,
) error {
	const sqlStatement = `INSERT INTO magic_token (
            email,        -- 1
            token,        -- 2
            action,       -- 3
            expires_at    -- 4
        ) VALUES (
            ?,            -- 1
            ?,            -- 2
            ?,            -- 3
            ?             -- 4
        );`

	return s.Exec(
		sqlStatement,
		email,     // 1
		token,     // 2
		action,    // 3
		expiresAt, // 4
	)
}

// ConsumeToken validates and consumes a single-use token, returning the
// email it was issued for. The action must match the one used at creation.
// Returns ("", nil) when the token does not exist, is expired, or has the
// wrong action — callers must treat the empty email as a failure.
func (s *SQLite) ConsumeToken(token, action string) (string, error) {
	const sqlSelect = `SELECT
            email
        FROM magic_token
        WHERE token = ?                       -- 1
        AND action = ?                        -- 2
        AND expires_at > CURRENT_TIMESTAMP
        LIMIT 1;`

	var email string
	err := s.QueryRow(
		sqlSelect,
		token,  // 1
		action, // 2
	).Scan(&email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}

	const sqlDelete = `DELETE FROM magic_token WHERE token = ?;` // 1

	err = s.Exec(
		sqlDelete,
		token, // 1
	)
	if err != nil {
		return "", err
	}

	return email, nil
}

// PurgeExpiredTokens removes expired single-use tokens regardless of action.
func (s *SQLite) PurgeExpiredTokens() error {
	const sqlStatement = `DELETE FROM magic_token
        WHERE expires_at <= CURRENT_TIMESTAMP;`

	return s.Exec(sqlStatement)
}
