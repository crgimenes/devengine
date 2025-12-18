package db

import (
	"errors"
	"fmt"
)

// and increments the record's rev for optimistic locking.
// Returns the new rev on success, or ErrConflict if currentRev doesn't match.
func (s *SQLite) UpsertEAVValueWithRev(
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
		return 0, fmt.Errorf("%w: exactly one value must be set", ErrInvalidValue)
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
			return 0, fmt.Errorf("%w: attribute %s requires BOOL value", ErrInvalidValue, attr.MachineName)
		}
	case "INT":
		if vInt == nil {
			return 0, fmt.Errorf("%w: attribute %s requires INT value", ErrInvalidValue, attr.MachineName)
		}
	case "REAL":
		if vReal == nil {
			return 0, fmt.Errorf("%w: attribute %s requires REAL value", ErrInvalidValue, attr.MachineName)
		}
	case "TEXT":
		if vText == nil {
			return 0, fmt.Errorf("%w: attribute %s requires TEXT value", ErrInvalidValue, attr.MachineName)
		}
	case "DATETIME":
		if vDatetime == nil {
			return 0, fmt.Errorf("%w: attribute %s requires DATETIME value", ErrInvalidValue, attr.MachineName)
		}
	default:
		return 0, fmt.Errorf("%w: unknown primitive_kind %s", ErrInvalidValue, attr.PrimitiveKind)
	}

	// Begin transaction for atomic update
	tx, err := s.rw.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Update record rev with optimistic lock check
	const sqlUpdateRev = `UPDATE eav_records
	SET
		rev = rev + 1,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ? AND rev = ? AND deleted_at IS NULL
	RETURNING rev;`

	var newRev int
	err = tx.QueryRow(sqlUpdateRev, recordID, currentRev).Scan(&newRev)
	if err != nil {
		if errors.Is(err, ErrNoRows) {
			return 0, ErrConflict
		}
		return 0, err
	}

	// Upsert value
	const sqlUpsert = `INSERT INTO eav_values (
		record_id,
		attribute_id,
		v_bool,
		v_int,
		v_real,
		v_text,
		v_datetime,
		updated_at
	) VALUES (
		?, ?, ?, ?, ?, ?, ?,
		CURRENT_TIMESTAMP
	) ON CONFLICT(record_id, attribute_id) DO UPDATE SET
		v_bool = excluded.v_bool,
		v_int = excluded.v_int,
		v_real = excluded.v_real,
		v_text = excluded.v_text,
		v_datetime = excluded.v_datetime,
		updated_at = CURRENT_TIMESTAMP;`

	_, err = tx.Exec(sqlUpsert,
		recordID,
		attributeID,
		vBool,
		vInt,
		vReal,
		vText,
		vDatetime,
	)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return newRev, nil
}
