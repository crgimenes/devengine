package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/crgimenes/devengine/filoeav"
)

// FieldFilter represents a filter condition for EAV queries.
// This type matches filoeav.FieldFilter to satisfy the EAVStore interface.
type FieldFilter struct {
	FieldMachineName string
	Operator         string // "=", "!=", "<", "<=", ">", ">="
	Value            any
}

// GetFieldValue retrieves a single field value from an EAV record.
// Returns nil if record or field not found.
func (s *SQLite) GetFieldValue(ctx context.Context, workspaceID int64, formSlug, fieldMachineName string, recordID int64) (any, error) {
	// First resolve form and field IDs from slugs/machine_names
	const sqlGetIDs = `
SELECT
  f.id,                   -- 1: form_id
  fld.id,                 -- 2: field_id
  fld.primitive_kind      -- 3: primitive_kind
FROM eav_forms AS f
JOIN eav_fields AS fld ON fld.form_id = f.id
WHERE f.workspace_id = ?          -- 1
  AND f.machine_name = ?          -- 2
  AND fld.machine_name = ?        -- 3
  AND fld.visible = 1`

	var formID, fieldID int64
	var primitiveKind string

	err := s.QueryRow(sqlGetIDs,
		workspaceID,      // 1
		formSlug,         // 2
		fieldMachineName, // 3
	).Scan(
		&formID,        // 1
		&fieldID,       // 2
		&primitiveKind, // 3
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("form %q or field %q not found", formSlug, fieldMachineName)
	}
	if err != nil {
		return nil, err
	}

	// Now get the value based on primitive_kind
	const sqlGetValue = `
SELECT
  value_bool,      -- 1
  value_datetime,  -- 2
  value_float,     -- 3
  value_int,       -- 4
  value_text       -- 5
FROM eav_values
WHERE record_id = ?   -- 1
  AND field_id = ?    -- 2`

	var valBool sql.NullBool
	var valDatetime sql.NullString
	var valFloat sql.NullFloat64
	var valInt sql.NullInt64
	var valText sql.NullString

	err = s.QueryRow(sqlGetValue,
		recordID, // 1
		fieldID,  // 2
	).Scan(
		&valBool,     // 1
		&valDatetime, // 2
		&valFloat,    // 3
		&valInt,      // 4
		&valText,     // 5
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // No value stored
	}
	if err != nil {
		return nil, err
	}

	// Return the appropriate value based on primitive_kind
	switch primitiveKind {
	case "BOOL":
		if valBool.Valid {
			return valBool.Bool, nil
		}
	case "DATETIME":
		if valDatetime.Valid {
			return valDatetime.String, nil // Return as string, caller can parse
		}
	case "FLOAT":
		if valFloat.Valid {
			return valFloat.Float64, nil
		}
	case "INT":
		if valInt.Valid {
			return valInt.Int64, nil
		}
	case "TEXT":
		if valText.Valid {
			return valText.String, nil
		}
	}

	return nil, nil
}

// FindRecordByField finds the first record where field matches value.
func (s *SQLite) FindRecordByField(ctx context.Context, workspaceID int64, formSlug, fieldMachineName, operator string, value any) (int64, error) {
	// Validate operator
	if !isValidSQLOperator(operator) {
		return 0, fmt.Errorf("invalid operator: %s", operator)
	}

	// Resolve form and field
	const sqlGetIDs = `
SELECT
  f.id,               -- 1: form_id
  fld.id,             -- 2: field_id
  fld.primitive_kind  -- 3: primitive_kind
FROM eav_forms AS f
JOIN eav_fields AS fld ON fld.form_id = f.id
WHERE f.workspace_id = ?          -- 1
  AND f.machine_name = ?          -- 2
  AND fld.machine_name = ?        -- 3
  AND fld.visible = 1`

	var formID, fieldID int64
	var primitiveKind string

	err := s.QueryRow(sqlGetIDs,
		workspaceID,      // 1
		formSlug,         // 2
		fieldMachineName, // 3
	).Scan(
		&formID,        // 1
		&fieldID,       // 2
		&primitiveKind, // 3
	)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("form %q or field %q not found", formSlug, fieldMachineName)
	}
	if err != nil {
		return 0, err
	}

	// Build the value column name based on primitive_kind
	valueColumn := primitiveKindToColumn(primitiveKind)

	// Build query with operator (safe because operator is validated)
	sqlFind := fmt.Sprintf(`
SELECT r.id
FROM eav_records AS r
JOIN eav_values AS v ON v.record_id = r.id
WHERE r.form_id = ?
  AND r.deleted_at IS NULL
  AND v.field_id = ?
  AND v.%s %s ?
LIMIT 1`, valueColumn, operator)

	var recordID int64
	err = s.QueryRow(sqlFind,
		formID,  // 1
		fieldID, // 2
		value,   // 3
	).Scan(&recordID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil // Not found
	}
	if err != nil {
		return 0, err
	}

	return recordID, nil
}

// AggregateField performs an aggregation on a field with optional filters.
func (s *SQLite) AggregateField(ctx context.Context, workspaceID int64, formSlug, fieldMachineName, aggFunc string, filters []filoeav.FieldFilter) (float64, error) {
	// Validate aggregation function
	if !isValidAggFunc(aggFunc) {
		return 0, fmt.Errorf("invalid aggregation function: %s", aggFunc)
	}

	// Resolve form ID
	const sqlGetFormID = `
SELECT id
FROM eav_forms
WHERE workspace_id = ?
  AND machine_name = ?`

	var formID int64
	err := s.QueryRow(sqlGetFormID,
		workspaceID, // 1
		formSlug,    // 2
	).Scan(&formID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("form %q not found", formSlug)
	}
	if err != nil {
		return 0, err
	}

	// For COUNT, we might not need a specific field
	var valueColumn string
	var targetFieldID int64

	if fieldMachineName != "" {
		// Resolve field
		const sqlGetField = `
SELECT id, primitive_kind
FROM eav_fields
WHERE form_id = ?
  AND machine_name = ?
  AND visible = 1`

		var primitiveKind string
		err = s.QueryRow(sqlGetField,
			formID,           // 1
			fieldMachineName, // 2
		).Scan(&targetFieldID, &primitiveKind)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("field %q not found", fieldMachineName)
		}
		if err != nil {
			return 0, err
		}

		valueColumn = primitiveKindToColumn(primitiveKind)
	}

	// Build the aggregation query
	var query strings.Builder
	args := make([]any, 0, 2+len(filters)*2)

	if aggFunc == "COUNT" && fieldMachineName == "" {
		// Count all records in form
		query.WriteString(`
SELECT COUNT(*)
FROM eav_records AS r
WHERE r.form_id = ?
  AND r.deleted_at IS NULL`)
		args = append(args, formID)
	} else {
		// Aggregate on specific field
		query.WriteString(fmt.Sprintf(`
SELECT COALESCE(%s(v.%s), 0)
FROM eav_records AS r
JOIN eav_values AS v ON v.record_id = r.id
WHERE r.form_id = ?
  AND r.deleted_at IS NULL
  AND v.field_id = ?`, aggFunc, valueColumn))
		args = append(args, formID, targetFieldID)
	}

	// Add filter conditions
	if len(filters) > 0 {
		// Resolve filter field IDs and build conditions
		for i, f := range filters {
			if !isValidSQLOperator(f.Operator) {
				return 0, fmt.Errorf("invalid operator in filter[%d]: %s", i, f.Operator)
			}

			// Get filter field info
			const sqlGetFilterField = `
SELECT id, primitive_kind
FROM eav_fields
WHERE form_id = ?
  AND machine_name = ?
  AND visible = 1`

			var filterFieldID int64
			var filterPrimitiveKind string
			err = s.QueryRow(sqlGetFilterField,
				formID,             // 1
				f.FieldMachineName, // 2
			).Scan(&filterFieldID, &filterPrimitiveKind)
			if errors.Is(err, sql.ErrNoRows) {
				return 0, fmt.Errorf("filter field %q not found", f.FieldMachineName)
			}
			if err != nil {
				return 0, err
			}

			filterColumn := primitiveKindToColumn(filterPrimitiveKind)
			query.WriteString(fmt.Sprintf(`
  AND r.id IN (
    SELECT record_id FROM eav_values
    WHERE field_id = ? AND %s %s ?
  )`, filterColumn, f.Operator))
			args = append(args, filterFieldID, f.Value)
		}
	}

	var result float64
	err = s.QueryRow(query.String(), args...).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	return result, nil
}

// primitiveKindToColumn maps EAV primitive_kind to the value column name.
func primitiveKindToColumn(kind string) string {
	switch kind {
	case "BOOL":
		return "value_bool"
	case "DATETIME":
		return "value_datetime"
	case "FLOAT":
		return "value_float"
	case "INT":
		return "value_int"
	case "TEXT":
		return "value_text"
	default:
		return "value_text"
	}
}

// isValidSQLOperator checks if the operator is safe for SQL.
func isValidSQLOperator(op string) bool {
	switch op {
	case "=", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

// isValidAggFunc checks if the aggregation function is valid.
func isValidAggFunc(fn string) bool {
	switch fn {
	case "SUM", "COUNT", "MIN", "MAX", "AVG":
		return true
	default:
		return false
	}
}
