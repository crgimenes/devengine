package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/crgimenes/devengine/log"
)

// SaveFileMetadata saves file metadata to the database.
func (s *SQLite) SaveFileMetadata(f *File) (*File, error) {
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
            ?,                     -- 1
            ?,                     -- 2
            ?,                     -- 3
            ?,                     -- 4
            ?,                     -- 5
            ?,                     -- 6
            ?,                     -- 7
            ?,                     -- 8
            ?                      -- 9
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

	var savedFile File
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
func (s *SQLite) GetFileByUserIDAndFilename(
	userID int64,
	filename string,
) (*File, error) {
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
    WHERE user_id = ?     -- 1
    AND filename = ?      -- 2
    AND deleted = 0
        LIMIT 1;`

	var f File
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
			return nil, nil // File not found
		}
		return nil, err
	}

	return &f, nil
}

// GetFileByUserReferenceIDAndFilename retrieves a file by user reference ID and filename.
func (s *SQLite) GetFileByUserReferenceIDAndFilename(
	userRefID string,
	filename string,
) (*File, error) {
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
    WHERE u.reference_id = ?  -- 1
    AND f.filename = ?        -- 2
    AND f.deleted = 0
        LIMIT 1;`

	var f File
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
			return nil, nil // File not found
		}
		return nil, err
	}

	return &f, nil
}

// ListFilesByUserID returns files for a user with pagination.
func (s *SQLite) ListFilesByUserID(
	userID int64,
	offset int,
	limit int,
) ([]*File, error) {
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
    WHERE user_id = ?        -- 1
    AND deleted = 0
        ORDER BY created_at DESC
        LIMIT ?                  -- 2
        OFFSET ?;                -- 3`
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

	var files []*File
	for rows.Next() {
		var f File
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
func (s *SQLite) CountFilesByUserID(userID int64) (int, error) {
	const sqlCount = `SELECT COUNT(*)
        FROM filemanager_files
        WHERE user_id = ?
        AND deleted = 0`

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
func (s *SQLite) ListFilesByUserIDSorted(
	userID int64,
	sort string,
	offset int,
	limit int,
) ([]*File, error) {
	orderBy := "created_at DESC"
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
        WHERE user_id = ? AND deleted = 0
        ORDER BY %s
        LIMIT ? OFFSET ?;`, orderBy)

	rows, err := s.Query(query, userID, limit, offset)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// Client aborted/canceled; do not log as error
			return nil, err
		}
		log.Println("ListFilesByUserIDSorted query error:", err)
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		var f File
		if err := rows.Scan(
			&f.ID,
			&f.UserID,
			&f.OriginalFilename,
			&f.Filename,
			&f.Filesize,
			&f.Filetype,
			&f.Filehash,
			&f.Filetag,
			&f.Filedescription,
			&f.Processed,
			&f.CreatedAt,
			&f.UpdatedAt,
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

// buildFTSQuery converts a raw user query into a safe FTS5 query by
// splitting on whitespace and joining tokens with AND and a trailing * for prefix match.
func buildFTSQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	parts := strings.Fields(q)
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// strip simple quotes to avoid breaking the MATCH syntax
		p = strings.ReplaceAll(p, "\"", "")
		p = strings.ReplaceAll(p, "'", "")
		tokens = append(tokens, p+"*")
	}
	if len(tokens) == 0 {
		return ""
	}
	// AND all tokens to narrow results
	return strings.Join(tokens, " ")
}

// SearchFilesByUserIDFTS performs a full-text search across original_filename, filename,
// filetag, and filedescription using the FTS virtual table.
// Returns paginated results for a given user.
// Sort supports: "date_desc" (default) and "name_asc".
func (s *SQLite) SearchFilesByUserIDFTS(
	userID int64,
	query string,
	sort string,
	offset int,
	limit int,
) ([]*File, error) {
	q := buildFTSQuery(query)
	if q == "" {
		// fallback to default listing if no query
		return s.ListFilesByUserID(userID, offset, limit)
	}

	orderBy := "f.created_at DESC"
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
        JOIN filemanager_files_fts ON filemanager_files_fts.rowid = f.id
        WHERE f.user_id = ?                    -- 1
		AND f.deleted = 0
		AND filemanager_files_fts MATCH ?      -- 2
        ORDER BY %s
        LIMIT ? OFFSET ?` // 3,4

	querySQL := fmt.Sprintf(baseSelect, orderBy)

	rows, err := s.Query(
		querySQL,
		userID, // 1
		q,      // 2
		limit,  // 3
		offset) // 4
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// Client aborted/canceled; do not log as error
			return nil, err
		}
		log.Println("SearchFilesByUserIDFTS query error:", err)
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		var f File
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
func (s *SQLite) SoftDeleteFileByUserAndFilename(
	userID int64,
	filename string,
) error {
	const sqlUpdate = `UPDATE filemanager_files
            SET deleted = 1,
                updated_at = CURRENT_TIMESTAMP
            WHERE filename = ? AND user_id = ? AND deleted = 0`
	return s.Exec(sqlUpdate, filename, userID)
}

// UpdateFileMetadataByUserAndFilename updates the file description and tag for a file
// owned by the given user, only if it is not deleted.
func (s *SQLite) UpdateFileMetadataByUserAndFilename(
	userID int64,
	filename string,
	description string,
	tag string,
) error {
	const sqlUpdate = `UPDATE filemanager_files
            SET filedescription = ?,
                filetag = ?,
                updated_at = CURRENT_TIMESTAMP
            WHERE filename = ? AND user_id = ? AND deleted = 0`
	return s.Exec(sqlUpdate, description, tag, filename, userID)
}
