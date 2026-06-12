package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/log"
)

// SaveFileMetadata saves file metadata to the database.
func (s *Postgres) SaveFileMetadata(f *db.File) (*db.File, error) {
	if f == nil {
		return nil, errors.New("file metadata is required")
	}

	const sqlInsert = `INSERT INTO filemanager_files (
            user_id,               -- 1
            original_filename,     -- 2
            filename,              -- 3
            filesize,              -- 4
            filetype,              -- 5
            filehash,              -- 6
            filetag,               -- 7
            filedescription,       -- 8
            processed              -- 9
        ) VALUES (
            $1,                     -- 1
            $2,                     -- 2
            $3,                     -- 3
            $4,                     -- 4
            $5,                     -- 5
            $6,                     -- 6
            $7,                     -- 7
            $8,                     -- 8
            $9                      -- 9
        )
        RETURNING
            id,                    -- 1
            user_id,               -- 2
            original_filename,     -- 3
            filename,              -- 4
            filesize,              -- 5
            filetype,              -- 6
            filehash,              -- 7
            filetag,               -- 8
            filedescription,       -- 9
            processed,             -- 10
            created_at,            -- 11
            updated_at;` // 12

	var savedFile db.File
	err := s.QueryRowRW(
		sqlInsert,
		f.UserID,           // 1
		f.OriginalFilename, // 2
		f.Filename,         // 3
		f.Filesize,         // 4
		f.Filetype,         // 5
		f.Filehash,         // 6
		f.Filetag,          // 7
		f.Filedescription,  // 8
		f.Processed,        // 9
	).Scan(
		&savedFile.ID,               // 1
		&savedFile.UserID,           // 2
		&savedFile.OriginalFilename, // 3
		&savedFile.Filename,         // 4
		&savedFile.Filesize,         // 5
		&savedFile.Filetype,         // 6
		&savedFile.Filehash,         // 7
		&savedFile.Filetag,          // 8
		&savedFile.Filedescription,  // 9
		&savedFile.Processed,        // 10
		&savedFile.CreatedAt,        // 11
		&savedFile.UpdatedAt,        // 12
	)
	if err != nil {
		return nil, err
	}

	return &savedFile, nil
}

// GetFileByUserIDAndFilename retrieves a file by user ID and filename.
func (s *Postgres) GetFileByUserIDAndFilename(
	userID int64,
	filename string,
) (*db.File, error) {
	const sqlSelect = `SELECT
            id,                    -- 1
            user_id,               -- 2
            original_filename,     -- 3
            filename,              -- 4
            filesize,              -- 5
            filetype,              -- 6
            filehash,              -- 7
            filetag,               -- 8
            filedescription,       -- 9
            processed,             -- 10
            created_at,            -- 11
            updated_at             -- 12
    FROM filemanager_files
    WHERE user_id = $1     -- 1
    AND filename = $2      -- 2
    AND deleted = FALSE
        LIMIT 1;`

	var f db.File
	err := s.QueryRow(
		sqlSelect,
		userID,   // 1
		filename, // 2
	).Scan(
		&f.ID,               // 1
		&f.UserID,           // 2
		&f.OriginalFilename, // 3
		&f.Filename,         // 4
		&f.Filesize,         // 5
		&f.Filetype,         // 6
		&f.Filehash,         // 7
		&f.Filetag,          // 8
		&f.Filedescription,  // 9
		&f.Processed,        // 10
		&f.CreatedAt,        // 11
		&f.UpdatedAt,        // 12
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // db.File not found
		}
		return nil, err
	}

	return &f, nil
}

// GetFileByFilename retrieves a non-deleted file by its opaque filename. The
// filename column is globally unique so no user scoping is needed.
func (s *Postgres) GetFileByFilename(filename string) (*db.File, error) {
	const sqlSelect = `SELECT
            id,                    -- 1
            user_id,               -- 2
            original_filename,     -- 3
            filename,              -- 4
            filesize,              -- 5
            filetype,              -- 6
            filehash,              -- 7
            filetag,               -- 8
            filedescription,       -- 9
            processed,             -- 10
            created_at,            -- 11
            updated_at             -- 12
    FROM filemanager_files
    WHERE filename = $1    -- 1
    AND deleted = FALSE
        LIMIT 1;`

	var f db.File
	err := s.QueryRow(
		sqlSelect,
		filename, // 1
	).Scan(
		&f.ID,               // 1
		&f.UserID,           // 2
		&f.OriginalFilename, // 3
		&f.Filename,         // 4
		&f.Filesize,         // 5
		&f.Filetype,         // 6
		&f.Filehash,         // 7
		&f.Filetag,          // 8
		&f.Filedescription,  // 9
		&f.Processed,        // 10
		&f.CreatedAt,        // 11
		&f.UpdatedAt,        // 12
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

// GetFileByUserReferenceIDAndFilename retrieves a file by user reference ID and filename.
func (s *Postgres) GetFileByUserReferenceIDAndFilename(
	userRefID string,
	filename string,
) (*db.File, error) {
	const sqlSelect = `SELECT
            f.id,                    -- 1
            f.user_id,               -- 2
            f.original_filename,     -- 3
            f.filename,              -- 4
            f.filesize,              -- 5
            f.filetype,              -- 6
            f.filehash,              -- 7
            f.filetag,               -- 8
            f.filedescription,       -- 9
            f.processed,             -- 10
            f.created_at,            -- 11
            f.updated_at             -- 12
        FROM filemanager_files f
        JOIN users u ON f.user_id = u.id
    WHERE u.reference_id = $1  -- 1
    AND f.filename = $2        -- 2
    AND f.deleted = FALSE
        LIMIT 1;`

	var f db.File
	err := s.QueryRow(
		sqlSelect,
		userRefID, // 1
		filename,  // 2
	).Scan(
		&f.ID,               // 1
		&f.UserID,           // 2
		&f.OriginalFilename, // 3
		&f.Filename,         // 4
		&f.Filesize,         // 5
		&f.Filetype,         // 6
		&f.Filehash,         // 7
		&f.Filetag,          // 8
		&f.Filedescription,  // 9
		&f.Processed,        // 10
		&f.CreatedAt,        // 11
		&f.UpdatedAt,        // 12
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // db.File not found
		}
		return nil, err
	}

	return &f, nil
}

// ListFilesByUserID returns files for a user with pagination.
func (s *Postgres) ListFilesByUserID(
	userID int64,
	offset int,
	limit int,
) ([]*db.File, error) {
	const sqlSelect = `SELECT
            id,                    -- 1
            user_id,               -- 2
            original_filename,     -- 3
            filename,              -- 4
            filesize,              -- 5
            filetype,              -- 6
            filehash,              -- 7
            filetag,               -- 8
            filedescription,       -- 9
            processed,             -- 10
            created_at,            -- 11
            updated_at             -- 12
    FROM filemanager_files
    WHERE user_id = $1        -- 1
    AND deleted = FALSE
        ORDER BY created_at DESC
        LIMIT $2                  -- 2
        OFFSET $3;                -- 3`
	rows, err := s.Query(
		sqlSelect,
		userID, // 1
		limit,  // 2
		offset, // 3
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// Client aborted/canceled; do not log as error
			return nil, err
		}
		log.Println("ListFilesByUserID query error:", err)
		return nil, err
	}
	defer rows.Close()

	var files []*db.File
	for rows.Next() {
		var f db.File
		err := rows.Scan(
			&f.ID,               // 1
			&f.UserID,           // 2
			&f.OriginalFilename, // 3
			&f.Filename,         // 4
			&f.Filesize,         // 5
			&f.Filetype,         // 6
			&f.Filehash,         // 7
			&f.Filetag,          // 8
			&f.Filedescription,  // 9
			&f.Processed,        // 10
			&f.CreatedAt,        // 11
			&f.UpdatedAt,        // 12
		)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				// Client aborted/canceled; do not log as error
				return nil, err
			}
			log.Println("ListFilesByUserID scan error:", err)
			return nil, err
		}
		files = append(files, &f)
	}

	return files, nil
}

// CountFilesByUserID returns the total count of non-deleted files for a user.
func (s *Postgres) CountFilesByUserID(userID int64) (int, error) {
	const sqlCount = `SELECT COUNT(*)
        FROM filemanager_files
        WHERE user_id = $1
        AND deleted = FALSE`

	var count int
	err := s.QueryRow(sqlCount, userID).Scan(&count)
	if err != nil {
		log.Println("CountFilesByUserID error:", err)
		return 0, err
	}

	return count, nil
}

// ListFilesByUserIDSorted returns files for a user with explicit sort option.
// sort supports: "date_desc" (default) and "name_asc".
func (s *Postgres) ListFilesByUserIDSorted(
	userID int64,
	sort string,
	offset int,
	limit int,
) ([]*db.File, error) {
	var orderBy string
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "name_asc":
		orderBy = "LOWER(original_filename) ASC, created_at DESC"
	default:
		orderBy = "created_at DESC"
	}

	query := fmt.Sprintf(`SELECT
            id,                    -- 1
            user_id,               -- 2
            original_filename,     -- 3
            filename,              -- 4
            filesize,              -- 5
            filetype,              -- 6
            filehash,              -- 7
            filetag,               -- 8
            filedescription,       -- 9
            processed,             -- 10
            created_at,            -- 11
            updated_at             -- 12
        FROM filemanager_files
        WHERE user_id = $1 AND deleted = FALSE
        ORDER BY %s
        LIMIT $2 OFFSET $3;`, orderBy)

	rows, err := s.Query(
		query,
		userID, // 1
		limit,  // 2
		offset, // 3
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// Client aborted/canceled; do not log as error
			return nil, err
		}
		log.Println("ListFilesByUserIDSorted query error:", err)
		return nil, err
	}
	defer rows.Close()

	var files []*db.File
	for rows.Next() {
		var f db.File
		if err := rows.Scan(
			&f.ID,               // 1
			&f.UserID,           // 2
			&f.OriginalFilename, // 3
			&f.Filename,         // 4
			&f.Filesize,         // 5
			&f.Filetype,         // 6
			&f.Filehash,         // 7
			&f.Filetag,          // 8
			&f.Filedescription,  // 9
			&f.Processed,        // 10
			&f.CreatedAt,        // 11
			&f.UpdatedAt,        // 12
		); err != nil {
			if errors.Is(err, context.Canceled) {
				// Client aborted/canceled; do not log as error
				return nil, err
			}
			log.Println("ListFilesByUserIDSorted scan error:", err)
			return nil, err
		}
		files = append(files, &f)
	}
	return files, nil
}

// buildFTSQuery converts a raw user query into a safe tsquery string:
// tokens are stripped to letters/digits, suffixed with :* for prefix match
// and joined with & (all tokens must match), mirroring the SQLite FTS5
// prefix semantics.
func buildFTSQuery(q string) string {
	parts := strings.Fields(strings.TrimSpace(q))
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		var clean strings.Builder
		for _, r := range p {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				clean.WriteRune(r)
			}
		}
		if clean.Len() == 0 {
			continue
		}
		tokens = append(tokens, clean.String()+":*")
	}
	return strings.Join(tokens, " & ")
}

// SearchFilesByUserIDFTS performs a full-text search across original_filename, filename,
// filetag, and filedescription using the generated tsvector column.
// Returns paginated results for a given user.
// Sort supports: "date_desc" (default) and "name_asc".
func (s *Postgres) SearchFilesByUserIDFTS(
	userID int64,
	query string,
	sort string,
	offset int,
	limit int,
) ([]*db.File, error) {
	q := buildFTSQuery(query)
	if q == "" {
		// fallback to default listing if no query
		return s.ListFilesByUserID(userID, offset, limit)
	}

	var orderBy string
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "name_asc":
		orderBy = "LOWER(f.original_filename) ASC, f.created_at DESC"
	default:
		orderBy = "f.created_at DESC"
	}

	const baseSelect = `SELECT
            f.id,                 -- 1
            f.user_id,            -- 2
            f.original_filename,  -- 3
            f.filename,           -- 4
            f.filesize,           -- 5
            f.filetype,           -- 6
            f.filehash,           -- 7
            f.filetag,            -- 8
            f.filedescription,    -- 9
            f.processed,          -- 10
            f.created_at,         -- 11
            f.updated_at          -- 12
        FROM filemanager_files f
        WHERE f.user_id = $1                       -- 1
        AND f.deleted = FALSE
        AND f.fts @@ to_tsquery('simple', $2)      -- 2
        ORDER BY %s
        LIMIT $3 OFFSET $4` // 3,4

	querySQL := fmt.Sprintf(baseSelect, orderBy)

	rows, err := s.Query(
		querySQL,
		userID, // 1
		q,      // 2
		limit,  // 3
		offset, // 4
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// Client aborted/canceled; do not log as error
			return nil, err
		}
		log.Println("SearchFilesByUserIDFTS query error:", err)
		return nil, err
	}
	defer rows.Close()

	var files []*db.File
	for rows.Next() {
		var f db.File
		err := rows.Scan(
			&f.ID,               // 1
			&f.UserID,           // 2
			&f.OriginalFilename, // 3
			&f.Filename,         // 4
			&f.Filesize,         // 5
			&f.Filetype,         // 6
			&f.Filehash,         // 7
			&f.Filetag,          // 8
			&f.Filedescription,  // 9
			&f.Processed,        // 10
			&f.CreatedAt,        // 11
			&f.UpdatedAt,        // 12
		)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				// Client aborted/canceled; do not log as error
				return nil, err
			}
			log.Println("SearchFilesByUserIDFTS scan error:", err)
			return nil, err
		}
		files = append(files, &f)
	}

	return files, nil
}

// SoftDeleteFileByUserAndFilename marks a file as deleted (soft delete) for a specific user and filename.
// It does not remove file contents from disk; a background worker may perform physical deletion later.
func (s *Postgres) SoftDeleteFileByUserAndFilename(
	userID int64,
	filename string,
) error {
	const sqlUpdate = `UPDATE filemanager_files
        SET
            deleted = TRUE,
            updated_at = CURRENT_TIMESTAMP
        WHERE filename = $1  -- 1
        AND user_id = $2     -- 2
        AND deleted = FALSE`
	return s.Exec(
		sqlUpdate,
		filename, // 1
		userID,   // 2
	)
}

// UpdateFileMetadataByUserAndFilename updates the file description and tag for a file
// owned by the given user, only if it is not deleted.
func (s *Postgres) UpdateFileMetadataByUserAndFilename(
	userID int64,
	filename string,
	description string,
	tag string,
) error {
	const sqlUpdate = `UPDATE filemanager_files
        SET
            filedescription = $1, -- 1
            filetag = $2,         -- 2
            updated_at = CURRENT_TIMESTAMP
        WHERE filename = $3       -- 3
        AND user_id = $4          -- 4
        AND deleted = FALSE`
	return s.Exec(
		sqlUpdate,
		description, // 1
		tag,         // 2
		filename,    // 3
		userID,      // 4
	)
}

// SumFileSizesByUserID returns the total bytes a user occupies on disk.
// Soft-deleted files COUNT: deletion only flags the row, the file stays on
// disk until a purge job exists, so the quota must see it.
func (s *Postgres) SumFileSizesByUserID(userID int64) (int64, error) {
	const sqlSelect = `SELECT COALESCE(SUM(filesize), 0)
        FROM filemanager_files
        WHERE user_id = $1`
	var total int64
	err := s.QueryRow(sqlSelect, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum file sizes: %w", err)
	}
	return total, nil
}
