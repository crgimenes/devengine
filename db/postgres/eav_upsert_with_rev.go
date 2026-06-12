package postgres

import (
	"errors"
	"fmt"

	"github.com/crgimenes/devengine/db"
)

// UpsertEAVValueWithRev inserts or updates a value for a (record, attribute) pair
// and increments the record's rev for optimistic locking.
// The rev increment validation is enforced by the trg_eav_records_update_rev trigger.
// Returns the new rev on success, or db.ErrConflict if currentRev doesn't match.
func (s *Postgres) UpsertEAVValueWithRev(
	recordID, attributeID int64,
	currentRev int,
	vBool *bool, vInt *int64, vReal *float64, vText, vDatetime *string,
) (int, error) {
	// Count non-nil values
	nonNilCount := 0
	if vBool != nil {
		nonNilCount++
	}
	if vInt != nil {
		nonNilCount++
	}
	if vReal != nil {
		nonNilCount++
	}
	if vText != nil {
		nonNilCount++
	}
	if vDatetime != nil {
		nonNilCount++
	}

	if nonNilCount != 1 {
		return 0, fmt.Errorf("%w: exactly one value must be set", db.ErrInvalidValue)
	}

	// Fetch attribute to validate type
	attr, err := s.GetEAVAttributeByID(attributeID)
	if err != nil {
		return 0, err
	}

	// Validate type matches
	switch attr.PrimitiveKind {
	case "BOOL":
		if vBool == nil {
			return 0, fmt.Errorf("%w: attribute %s requires BOOL value", db.ErrInvalidValue, attr.MachineName)
		}
	case "INT":
		if vInt == nil {
			return 0, fmt.Errorf("%w: attribute %s requires INT value", db.ErrInvalidValue, attr.MachineName)
		}
	case "REAL":
		if vReal == nil {
			return 0, fmt.Errorf("%w: attribute %s requires REAL value", db.ErrInvalidValue, attr.MachineName)
		}
	case "TEXT":
		if vText == nil {
			return 0, fmt.Errorf("%w: attribute %s requires TEXT value", db.ErrInvalidValue, attr.MachineName)
		}
	case "DATETIME":
		if vDatetime == nil {
			return 0, fmt.Errorf("%w: attribute %s requires DATETIME value", db.ErrInvalidValue, attr.MachineName)
		}
	default:
		return 0, fmt.Errorf("%w: unknown primitive_kind %s", db.ErrInvalidValue, attr.PrimitiveKind)
	}

	// Begin transaction for atomic update
	tx, err := s.pool.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Update record rev with optimistic lock check
	const sqlUpdateRev = `UPDATE eav_records
	SET
		rev = rev + 1,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = $1       -- 1
	AND rev = $2        -- 2
	AND deleted_at IS NULL
	RETURNING rev;`

	var newRev int
	err = tx.QueryRow(
		sqlUpdateRev,
		recordID,   // 1
		currentRev, // 2
	).Scan(&newRev)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return 0, db.ErrConflict
		}
		return 0, err
	}

	// Upsert value
	const sqlUpsert = `INSERT INTO eav_values (
		record_id,    -- 1
		attribute_id, -- 2
		v_bool,       -- 3
		v_int,        -- 4
		v_real,       -- 5
		v_text,       -- 6
		v_datetime,   -- 7
		updated_at
	) VALUES (
		$1,                 -- 1
		$2,                 -- 2
		$3,                 -- 3
		$4,                 -- 4
		$5,                 -- 5
		$6,                 -- 6
		$7,                 -- 7
		CURRENT_TIMESTAMP  -- updated_at
	) ON CONFLICT(record_id, attribute_id) DO UPDATE SET
		v_bool = excluded.v_bool,
		v_int = excluded.v_int,
		v_real = excluded.v_real,
		v_text = excluded.v_text,
		v_datetime = excluded.v_datetime,
		updated_at = CURRENT_TIMESTAMP;`

	_, err = tx.Exec(
		sqlUpsert,
		recordID,    // 1
		attributeID, // 2
		vBool,       // 3
		vInt,        // 4
		vReal,       // 5
		vText,       // 6
		vDatetime,   // 7
	)
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return newRev, nil
}
