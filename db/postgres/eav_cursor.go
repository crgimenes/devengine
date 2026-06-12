package postgres

import (
	"fmt"
	"strings"

	"github.com/crgimenes/devengine/db"
)

// ListEAVRecordsCursor returns up to limit records of entity_type ordered by
// id DESC, starting strictly after the given cursor id. Cursor 0 means "from
// the top". The optional textFilter narrows the result to records that have
// at least one TEXT value containing the substring (case-insensitive).
//
// Designed for infinite-scroll viewers: each subsequent call passes the id
// of the last record from the previous batch as the new cursor.
func (s *Postgres) ListEAVRecordsCursor(
	entityTypeID int64,
	cursorID int64,
	limit int,
	textFilter string,
) ([]db.EAVRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	var (
		query strings.Builder
		args  []any
	)
	// next appends an arg and returns its $N placeholder; keeps numbering
	// correct across the conditionally assembled pieces.
	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if textFilter == "" {
		query.WriteString(`
        SELECT id, reference_id, entity_type_id, status, rev,
               created_at, updated_at, COALESCE(deleted_at::text, '')
        FROM eav_records
        WHERE entity_type_id = ` + next(entityTypeID) + `
        AND deleted_at IS NULL`)
		if cursorID > 0 {
			query.WriteString(`
        AND id < ` + next(cursorID))
		}
	} else {
		// A record matches when one of its own TEXT values contains the
		// filter, OR when a TEXT value is the reference_id of a record
		// whose TEXT values contain it — so searching "Ana" finds the
		// orders pointing at Ana even though the column stores her opaque
		// id. One level of indirection only.
		pattern := "%" + textFilter + "%"
		query.WriteString(`
        SELECT DISTINCT r.id, r.reference_id, r.entity_type_id, r.status, r.rev,
                        r.created_at, r.updated_at, COALESCE(r.deleted_at::text, '')
        FROM eav_records r
        JOIN eav_values v ON v.record_id = r.id
        WHERE r.entity_type_id = ` + next(entityTypeID) + `
        AND r.deleted_at IS NULL
        AND v.v_text IS NOT NULL
        AND (
            LOWER(v.v_text) LIKE LOWER(` + next(pattern) + `)
            OR EXISTS (
                SELECT 1
                FROM eav_records ref_r
                JOIN eav_values ref_v ON ref_v.record_id = ref_r.id
                WHERE ref_r.reference_id = v.v_text
                AND ref_r.deleted_at IS NULL
                AND ref_v.v_text IS NOT NULL
                AND LOWER(ref_v.v_text) LIKE LOWER(` + next(pattern) + `)
            )
        )`)
		if cursorID > 0 {
			query.WriteString(`
        AND r.id < ` + next(cursorID))
		}
	}

	query.WriteString(`
        ORDER BY ` + alias(textFilter != "") + `id DESC
        LIMIT ` + next(limit))

	rows, err := s.Query(query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("ListEAVRecordsCursor: %w", err)
	}
	defer rows.Close()

	var out []db.EAVRecord
	for rows.Next() {
		var r db.EAVRecord
		var deletedAt string
		err := rows.Scan(
			&r.ID, &r.ReferenceID, &r.EntityTypeID, &r.Status, &r.Rev,
			&r.CreatedAt, &r.UpdatedAt, &deletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// alias returns "r." when the query joins eav_records as r, "" otherwise.
// Keeps the ORDER BY clause in ListEAVRecordsCursor consistent across both
// query shapes without duplicating the entire string.
func alias(joined bool) string {
	if joined {
		return "r."
	}
	return ""
}

// GetEAVValuesForRecordIDs loads all values for the given record ids in a
// single query and returns them keyed by record_id. Used by listing handlers
// to avoid N+1 round trips.
func (s *Postgres) GetEAVValuesForRecordIDs(ids []int64) (map[int64][]db.EAVValue, error) {
	if len(ids) == 0 {
		return map[int64][]db.EAVValue{}, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := `SELECT record_id, attribute_id, v_bool, v_int, v_real, v_text, v_datetime
        FROM eav_values
        WHERE record_id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := s.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("GetEAVValuesForRecordIDs: %w", err)
	}
	defer rows.Close()

	out := make(map[int64][]db.EAVValue, len(ids))
	for rows.Next() {
		var v db.EAVValue
		err := rows.Scan(&v.RecordID, &v.AttributeID,
			&v.VBool, &v.VInt, &v.VReal, &v.VText, &v.VDatetime)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out[v.RecordID] = append(out[v.RecordID], v)
	}
	return out, rows.Err()
}
