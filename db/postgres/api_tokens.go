package postgres

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/crgimenes/devengine/db"
)

// CreateAPIToken stores the hash of a freshly minted bearer token.
func (s *Postgres) CreateAPIToken(userID int64, tokenHash, label string) (*db.APIToken, error) {
	if tokenHash == "" {
		return nil, errors.New("token hash required")
	}
	const q = `INSERT INTO api_tokens (
        user_id,    -- 1
        token_hash, -- 2
        label       -- 3
    ) VALUES (
        $1, -- 1
        $2, -- 2
        $3  -- 3
    )
    RETURNING
        id,        -- 1
        user_id,   -- 2
        label,     -- 3
        created_at -- 4`

	var t db.APIToken
	err := s.QueryRowRW(
		q,
		userID,    // 1
		tokenHash, // 2
		label,     // 3
	).Scan(
		&t.ID,        // 1
		&t.UserID,    // 2
		&t.Label,     // 3
		&t.CreatedAt, // 4
	)
	if err != nil {
		return nil, fmt.Errorf("create api token: %w", err)
	}
	return &t, nil
}

// ListAPITokensByUserID returns the user's tokens, newest first.
func (s *Postgres) ListAPITokensByUserID(userID int64) ([]db.APIToken, error) {
	const q = `SELECT
        id,         -- 1
        user_id,    -- 2
        label,      -- 3
        created_at  -- 4
    FROM api_tokens
    WHERE user_id = $1 -- 1
    ORDER BY id DESC`

	rows, err := s.Query(q, userID)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()

	var out []db.APIToken
	for rows.Next() {
		var t db.APIToken
		err := rows.Scan(
			&t.ID,        // 1
			&t.UserID,    // 2
			&t.Label,     // 3
			&t.CreatedAt, // 4
		)
		if err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteAPIToken revokes a token. The userID guard keeps a user from
// deleting someone else's token.
func (s *Postgres) DeleteAPIToken(id, userID int64) error {
	const q = `DELETE FROM api_tokens
    WHERE id = $1      -- 1
    AND user_id = $2   -- 2`
	return s.Exec(
		q,
		id,     // 1
		userID, // 2
	)
}

// GetUserByAPITokenHash resolves a presented token hash to its (enabled)
// owner. Unknown hashes and disabled owners return nil, nil.
func (s *Postgres) GetUserByAPITokenHash(tokenHash string) (*db.User, error) {
	const q = `SELECT ` + userSelectColumns + `
    FROM users
    WHERE id = (SELECT user_id FROM api_tokens WHERE token_hash = $1) -- 1
    AND enabled = TRUE`

	u, err := scanUser(s.QueryRow(q, tokenHash))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by api token: %w", err)
	}
	return u, nil
}
