package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/utils"
)

// CreateEAVEntityType creates a new entity type with an opaque reference_id.
func (s *SQLite) CreateEAVEntityType(name, machineName, description, preSave, posLoad string) (*db.EAVEntityType, error) {
	refID := utils.NewOpaqueID()
	const sqlInsert = `INSERT INTO eav_entity_types (
		reference_id,  -- 1
		machine_name,  -- 2
		name,          -- 3
		description,   -- 4
		pre_save,      -- 5
		pos_load,      -- 6
		created_at,
		updated_at
	) VALUES (
		?,                 -- 1
		?,                 -- 2
		?,                 -- 3
		?,                 -- 4
		?,                 -- 5
		?,                 -- 6
		CURRENT_TIMESTAMP, -- created_at
		CURRENT_TIMESTAMP  -- updated_at
	) RETURNING
		id,                     -- 1
		reference_id,           -- 2
		machine_name,           -- 3
		name,                   -- 4
		description,            -- 5
		COALESCE(pre_save, ''), -- 6
		COALESCE(pos_load, ''), -- 7
		created_at,             -- 8
		updated_at              -- 9
	;`

	var et db.EAVEntityType
	err := s.QueryRowRW(sqlInsert,
		refID,       // 1
		machineName, // 2
		name,        // 3
		description, // 4
		preSave,     // 5
		posLoad,     // 6
	).Scan(
		&et.ID,          // 1
		&et.ReferenceID, // 2
		&et.MachineName, // 3
		&et.Name,        // 4
		&et.Description, // 5
		&et.PreSave,     // 6
		&et.PosLoad,     // 7
		&et.CreatedAt,   // 8
		&et.UpdatedAt,   // 9
	)
	if err != nil {
		return nil, err
	}
	return &et, nil
}

// GetEAVEntityTypeByID retrieves an entity type by internal ID (for internal use).
func (s *SQLite) GetEAVEntityTypeByID(id int64) (*db.EAVEntityType, error) {
	const sqlSelect = `SELECT
		id,                         -- 1
		reference_id,               -- 2
		machine_name,               -- 3
		name,                       -- 4
		COALESCE(description, ''),  -- 5
		COALESCE(pre_save, ''),     -- 6
		COALESCE(pos_load, ''),     -- 7
		created_at,                 -- 8
		updated_at                  -- 9
	FROM eav_entity_types
	WHERE id = ? AND deleted_at IS NULL;` // 1

	var et db.EAVEntityType
	err := s.QueryRow(sqlSelect,
		id, // 1
	).Scan(
		&et.ID,          // 1
		&et.ReferenceID, // 2
		&et.MachineName, // 3
		&et.Name,        // 4
		&et.Description, // 5
		&et.PreSave,     // 6
		&et.PosLoad,     // 7
		&et.CreatedAt,   // 8
		&et.UpdatedAt,   // 9
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, err
	}
	return &et, nil
}

// GetEAVEntityTypeByRefID retrieves an entity type by opaque reference_id (for external APIs).
func (s *SQLite) GetEAVEntityTypeByRefID(refID string) (*db.EAVEntityType, error) {
	const sqlSelect = `SELECT
		id,                         -- 1
		reference_id,               -- 2
		machine_name,               -- 3
		name,                       -- 4
		COALESCE(description, ''),  -- 5
		COALESCE(pre_save, ''),     -- 6
		COALESCE(pos_load, ''),     -- 7
		created_at,                 -- 8
		updated_at                  -- 9
	FROM eav_entity_types
	WHERE reference_id = ? AND deleted_at IS NULL;` // 1

	var et db.EAVEntityType
	err := s.QueryRow(sqlSelect,
		refID, // 1
	).Scan(
		&et.ID,          // 1
		&et.ReferenceID, // 2
		&et.MachineName, // 3
		&et.Name,        // 4
		&et.Description, // 5
		&et.PreSave,     // 6
		&et.PosLoad,     // 7
		&et.CreatedAt,   // 8
		&et.UpdatedAt,   // 9
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, err
	}
	return &et, nil
}

// GetEAVEntityTypeByMachineName retrieves an entity type by machine_name (for code/routing).
func (s *SQLite) GetEAVEntityTypeByMachineName(machineName string) (*db.EAVEntityType, error) {
	const sqlSelect = `SELECT
		id,                         -- 1
		reference_id,               -- 2
		machine_name,               -- 3
		name,                       -- 4
		COALESCE(description, ''),  -- 5
		COALESCE(pre_save, ''),     -- 6
		COALESCE(pos_load, ''),     -- 7
		created_at,                 -- 8
		updated_at                  -- 9
	FROM eav_entity_types
	WHERE LOWER(machine_name) = LOWER(?) AND deleted_at IS NULL;` // 1

	var et db.EAVEntityType
	err := s.QueryRow(sqlSelect,
		machineName, // 1
	).Scan(
		&et.ID,          // 1
		&et.ReferenceID, // 2
		&et.MachineName, // 3
		&et.Name,        // 4
		&et.Description, // 5
		&et.PreSave,     // 6
		&et.PosLoad,     // 7
		&et.CreatedAt,   // 8
		&et.UpdatedAt,   // 9
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, err
	}
	return &et, nil
}

// ListEAVEntityTypes returns all active entity types ordered by machine_name.
func (s *SQLite) ListEAVEntityTypes() ([]db.EAVEntityType, error) {
	const sqlSelect = `SELECT
		id,                         -- 1
		reference_id,               -- 2
		machine_name,               -- 3
		name,                       -- 4
		COALESCE(description, ''),  -- 5
		COALESCE(pre_save, ''),     -- 6
		COALESCE(pos_load, ''),     -- 7
		created_at,                 -- 8
		updated_at                  -- 9
	FROM eav_entity_types
	WHERE deleted_at IS NULL
	ORDER BY machine_name ASC;`

	rows, err := s.Query(sqlSelect)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.EAVEntityType
	for rows.Next() {
		var et db.EAVEntityType
		err := rows.Scan(
			&et.ID,          // 1
			&et.ReferenceID, // 2
			&et.MachineName, // 3
			&et.Name,        // 4
			&et.Description, // 5
			&et.PreSave,     // 6
			&et.PosLoad,     // 7
			&et.CreatedAt,   // 8
			&et.UpdatedAt,   // 9
		)
		if err != nil {
			return nil, err
		}
		list = append(list, et)
	}
	return list, rows.Err()
}

// UpdateEAVEntityType updates an existing entity type's metadata.
func (s *SQLite) UpdateEAVEntityType(id int64, name, description, preSave, posLoad string) (*db.EAVEntityType, error) {
	const sqlUpdate = `UPDATE eav_entity_types SET
		name = ?,        -- 1
		description = ?, -- 2
		pre_save = ?,    -- 3
		pos_load = ?,    -- 4
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ? AND deleted_at IS NULL
	RETURNING
		id,                         -- 1
		reference_id,               -- 2
		machine_name,               -- 3
		name,                       -- 4
		COALESCE(description, ''),  -- 5
		COALESCE(pre_save, ''),     -- 6
		COALESCE(pos_load, ''),     -- 7
		created_at,                 -- 8
		updated_at                  -- 9
	;`

	var et db.EAVEntityType
	err := s.QueryRowRW(sqlUpdate,
		name,        // 1
		description, // 2
		preSave,     // 3
		posLoad,     // 4
		id,          // 5 (WHERE clause)
	).Scan(
		&et.ID,          // 1
		&et.ReferenceID, // 2
		&et.MachineName, // 3
		&et.Name,        // 4
		&et.Description, // 5
		&et.PreSave,     // 6
		&et.PosLoad,     // 7
		&et.CreatedAt,   // 8
		&et.UpdatedAt,   // 9
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, err
	}
	return &et, nil
}

// SoftDeleteEAVEntityType marks an entity type as deleted.
func (s *SQLite) SoftDeleteEAVEntityType(id int64) error {
	const sqlUpdate = `UPDATE eav_entity_types
	SET deleted_at = CURRENT_TIMESTAMP
	WHERE id = ? AND deleted_at IS NULL;` // 1

	err := s.Exec(sqlUpdate, id)
	if err != nil {
		return err
	}
	return nil
}

// ====================================================================
// Attribute Operations
// ====================================================================

// CreateEAVAttribute creates a new attribute with validation and default values.
func (s *SQLite) CreateEAVAttribute(
	entityTypeID int64,
	machineName, label, helpText, primitiveKind string,
	isRequired, isUnique, isIndexed bool,
	maxLength *int, // For TEXT fields, NULL for other types
	isComputed bool,
	computedExpr string,
	defaultVBool *bool,
	defaultVInt *int64,
	defaultVReal *float64,
	defaultVText, defaultVDatetime *string,
) (*db.EAVAttribute, error) {
	// Validate primitive_kind
	validKinds := map[string]bool{
		"BOOL":     true,
		"INT":      true,
		"REAL":     true,
		"TEXT":     true,
		"DATETIME": true,
	}
	if !validKinds[primitiveKind] {
		return nil, fmt.Errorf("%w: primitive_kind must be one of BOOL, INT, REAL, TEXT, DATETIME", db.ErrInvalidValue)
	}

	refID := utils.NewOpaqueID()
	const sqlInsert = `INSERT INTO eav_attributes (
		reference_id,        -- 1
		entity_type_id,      -- 2
		machine_name,        -- 3
		label,               -- 4
		help_text,           -- 5
		primitive_kind,      -- 6
		is_required,         -- 7
		is_unique,           -- 8
		is_indexed,          -- 9
		max_length,          -- 10
		is_computed,         -- 11
		computed_expr,       -- 12
		default_v_bool,      -- 13
		default_v_int,       -- 14
		default_v_real,      -- 15
		default_v_text,      -- 16
		default_v_datetime,  -- 17
		created_at,
		updated_at
	) VALUES (
		?,                 -- 1
		?,                 -- 2
		?,                 -- 3
		?,                 -- 4
		?,                 -- 5
		?,                 -- 6
		?,                 -- 7
		?,                 -- 8
		?,                 -- 9
		?,                 -- 10
		?,                 -- 11
		?,                 -- 12
		?,                 -- 13
		?,                 -- 14
		?,                 -- 15
		?,                 -- 16
		?,                 -- 17
		CURRENT_TIMESTAMP, -- created_at
		CURRENT_TIMESTAMP  -- updated_at
	) RETURNING
		id,                  -- 1
		reference_id,        -- 2
		entity_type_id,      -- 3
		machine_name,        -- 4
		label,               -- 5
		help_text,           -- 6
		primitive_kind,      -- 7
		is_required,         -- 8
		is_unique,           -- 9
		is_indexed,          -- 10
		max_length,          -- 11
		is_computed,         -- 12
		computed_expr,       -- 13
		default_v_bool,      -- 14
		default_v_int,       -- 15
		default_v_real,      -- 16
		default_v_text,      -- 17
		default_v_datetime,  -- 18
		created_at,          -- 19
		updated_at           -- 20
	;`

	var attr db.EAVAttribute
	err := s.QueryRowRW(sqlInsert,
		refID,            // 1
		entityTypeID,     // 2
		machineName,      // 3
		label,            // 4
		helpText,         // 5
		primitiveKind,    // 6
		isRequired,       // 7
		isUnique,         // 8
		isIndexed,        // 9
		maxLength,        // 10
		isComputed,       // 11
		computedExpr,     // 12
		defaultVBool,     // 13
		defaultVInt,      // 14
		defaultVReal,     // 15
		defaultVText,     // 16
		defaultVDatetime, // 17
	).Scan(
		&attr.ID,               // 1
		&attr.ReferenceID,      // 2
		&attr.EntityTypeID,     // 3
		&attr.MachineName,      // 4
		&attr.Label,            // 5
		&attr.HelpText,         // 6
		&attr.PrimitiveKind,    // 7
		&attr.IsRequired,       // 8
		&attr.IsUnique,         // 9
		&attr.IsIndexed,        // 10
		&attr.MaxLength,        // 11
		&attr.IsComputed,       // 12
		&attr.ComputedExpr,     // 13
		&attr.DefaultVBool,     // 14
		&attr.DefaultVInt,      // 15
		&attr.DefaultVReal,     // 16
		&attr.DefaultVText,     // 17
		&attr.DefaultVDatetime, // 18
		&attr.CreatedAt,        // 19
		&attr.UpdatedAt,        // 20
	)
	if err != nil {
		return nil, err
	}
	return &attr, nil
}

// UpdateEAVAttribute updates an existing attribute including default values.
func (s *SQLite) UpdateEAVAttribute(
	id int64,
	machineName, label, helpText, primitiveKind string,
	isRequired, isUnique, isIndexed bool,
	maxLength *int, // For TEXT fields
	isComputed bool,
	computedExpr string,
	defaultVBool *bool,
	defaultVInt *int64,
	defaultVReal *float64,
	defaultVText, defaultVDatetime *string,
) (*db.EAVAttribute, error) {
	// Validate primitive_kind
	validKinds := map[string]bool{
		"BOOL":     true,
		"INT":      true,
		"REAL":     true,
		"TEXT":     true,
		"DATETIME": true,
	}
	if !validKinds[primitiveKind] {
		return nil, fmt.Errorf("%w: primitive_kind must be one of BOOL, INT, REAL, TEXT, DATETIME", db.ErrInvalidValue)
	}

	const sqlUpdate = `UPDATE eav_attributes SET
		machine_name = ?,        -- 1
		label = ?,               -- 2
		help_text = ?,           -- 3
		primitive_kind = ?,      -- 4
		is_required = ?,         -- 5
		is_unique = ?,           -- 6
		is_indexed = ?,          -- 7
		max_length = ?,          -- 8
		is_computed = ?,         -- 9
		computed_expr = ?,       -- 10
		default_v_bool = ?,      -- 11
		default_v_int = ?,       -- 12
		default_v_real = ?,      -- 13
		default_v_text = ?,      -- 14
		default_v_datetime = ?,  -- 15
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ? AND deleted_at IS NULL
	RETURNING
		id,                  -- 1
		reference_id,        -- 2
		entity_type_id,      -- 3
		machine_name,        -- 4
		label,               -- 5
		help_text,           -- 6
		primitive_kind,      -- 7
		is_required,         -- 8
		is_unique,           -- 9
		is_indexed,          -- 10
		max_length,          -- 11
		is_computed,         -- 12
		computed_expr,       -- 13
		default_v_bool,      -- 14
		default_v_int,       -- 15
		default_v_real,      -- 16
		default_v_text,      -- 17
		default_v_datetime,  -- 18
		created_at,          -- 19
		updated_at           -- 20
	`

	var attr db.EAVAttribute
	err := s.QueryRowRW(sqlUpdate,
		machineName,      // 1
		label,            // 2
		helpText,         // 3
		primitiveKind,    // 4
		isRequired,       // 5
		isUnique,         // 6
		isIndexed,        // 7
		maxLength,        // 8
		isComputed,       // 9
		computedExpr,     // 10
		defaultVBool,     // 11
		defaultVInt,      // 12
		defaultVReal,     // 13
		defaultVText,     // 14
		defaultVDatetime, // 15
		id,               // 16 (WHERE clause)
	).Scan(
		&attr.ID,               // 1
		&attr.ReferenceID,      // 2
		&attr.EntityTypeID,     // 3
		&attr.MachineName,      // 4
		&attr.Label,            // 5
		&attr.HelpText,         // 6
		&attr.PrimitiveKind,    // 7
		&attr.IsRequired,       // 8
		&attr.IsUnique,         // 9
		&attr.IsIndexed,        // 10
		&attr.MaxLength,        // 11
		&attr.IsComputed,       // 12
		&attr.ComputedExpr,     // 13
		&attr.DefaultVBool,     // 14
		&attr.DefaultVInt,      // 15
		&attr.DefaultVReal,     // 16
		&attr.DefaultVText,     // 17
		&attr.DefaultVDatetime, // 18
		&attr.CreatedAt,        // 19
		&attr.UpdatedAt,        // 20
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrNotFound
		}
		return nil, err
	}
	return &attr, nil
}

// CheckEAVValueUnique verifies if a value is unique for a given attribute.
// Returns true if the value is unique (or NULL), false if a duplicate exists.
//
// Parameters:
//   - attributeID: The attribute to check
//   - primitiveKind: Type of the attribute (BOOL, INT, REAL, TEXT, DATETIME)
//   - value: The value to check (must match primitiveKind type)
//   - excludeRecordID: Record ID to exclude from check (use 0 for new records)
//
// Rules:
//   - NULL values are always considered unique (multiple NULLs allowed)
//   - Only checks against active (non-deleted) records
//   - Uses read-only pool for optimal concurrency
func (s *SQLite) CheckEAVValueUnique(
	attributeID int64,
	primitiveKind string,
	value any,
	excludeRecordID int64,
) (bool, error) {
	// NULL values are always unique
	if value == nil {
		return true, nil
	}

	// Build query based on primitive kind
	var sqlCheck string
	var args []any

	switch primitiveKind {
	case "BOOL":
		sqlCheck = `SELECT COUNT(*) FROM eav_values v
			INNER JOIN eav_records r ON v.record_id = r.id
			WHERE v.attribute_id = ?
			  AND v.v_bool = ?
			  AND v.record_id != ?
			  AND r.deleted_at IS NULL`
		args = []any{attributeID, value, excludeRecordID}

	case "INT":
		sqlCheck = `SELECT COUNT(*) FROM eav_values v
			INNER JOIN eav_records r ON v.record_id = r.id
			WHERE v.attribute_id = ?
			  AND v.v_int = ?
			  AND v.record_id != ?
			  AND r.deleted_at IS NULL`
		args = []any{attributeID, value, excludeRecordID}

	case "REAL":
		sqlCheck = `SELECT COUNT(*) FROM eav_values v
			INNER JOIN eav_records r ON v.record_id = r.id
			WHERE v.attribute_id = ?
			  AND v.v_real = ?
			  AND v.record_id != ?
			  AND r.deleted_at IS NULL`
		args = []any{attributeID, value, excludeRecordID}

	case "TEXT":
		sqlCheck = `SELECT COUNT(*) FROM eav_values v
			INNER JOIN eav_records r ON v.record_id = r.id
			WHERE v.attribute_id = ?
			  AND v.v_text = ?
			  AND v.record_id != ?
			  AND r.deleted_at IS NULL`
		args = []any{attributeID, value, excludeRecordID}

	case "DATETIME":
		sqlCheck = `SELECT COUNT(*) FROM eav_values v
			INNER JOIN eav_records r ON v.record_id = r.id
			WHERE v.attribute_id = ?
			  AND v.v_datetime = ?
			  AND v.record_id != ?
			  AND r.deleted_at IS NULL`
		args = []any{attributeID, value, excludeRecordID}

	default:
		return false, fmt.Errorf("%w: unsupported primitive_kind: %s", db.ErrInvalidValue, primitiveKind)
	}

	var count int
	err := s.QueryRow(sqlCheck, args...).Scan(&count)
	if err != nil {
		return false, err
	}

	// Unique if count == 0
	return count == 0, nil
}

// GetEAVAttributeByID retrieves an attribute by internal ID.
func (s *SQLite) GetEAVAttributeByID(id int64) (*db.EAVAttribute, error) {
	const sqlSelect = `SELECT
		id,                          -- 1
		reference_id,                -- 2
		entity_type_id,              -- 3
		machine_name,                -- 4
		label,                       -- 5
		COALESCE(help_text, ''),     -- 6
		primitive_kind,              -- 7
		is_required,                 -- 8
		is_unique,                   -- 9
		is_indexed,                  -- 10
		max_length,                  -- 11
		is_computed,                 -- 12
		COALESCE(computed_expr, ''), -- 13
		default_v_bool,              -- 14
		default_v_int,               -- 15
		default_v_real,              -- 16
		default_v_text,              -- 17
		default_v_datetime,          -- 18
		created_at,                  -- 19
		updated_at                   -- 20
	FROM eav_attributes
	WHERE id = ? AND deleted_at IS NULL;` // 1

	var attr db.EAVAttribute
	err := s.QueryRow(sqlSelect,
		id, // 1
	).Scan(
		&attr.ID,               // 1
		&attr.ReferenceID,      // 2
		&attr.EntityTypeID,     // 3
		&attr.MachineName,      // 4
		&attr.Label,            // 5
		&attr.HelpText,         // 6
		&attr.PrimitiveKind,    // 7
		&attr.IsRequired,       // 8
		&attr.IsUnique,         // 9
		&attr.IsIndexed,        // 10
		&attr.MaxLength,        // 11
		&attr.IsComputed,       // 12
		&attr.ComputedExpr,     // 13
		&attr.DefaultVBool,     // 14
		&attr.DefaultVInt,      // 15
		&attr.DefaultVReal,     // 16
		&attr.DefaultVText,     // 17
		&attr.DefaultVDatetime, // 18
		&attr.CreatedAt,        // 19
		&attr.UpdatedAt,        // 20
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, err
	}
	return &attr, nil
}

// GetEAVAttributeByRefID retrieves an attribute by opaque reference_id.
func (s *SQLite) GetEAVAttributeByRefID(refID string) (*db.EAVAttribute, error) {
	const sqlSelect = `SELECT
		id,                          -- 1
		reference_id,                -- 2
		entity_type_id,              -- 3
		machine_name,                -- 4
		label,                       -- 5
		COALESCE(help_text, ''),     -- 6
		primitive_kind,              -- 7
		is_required,                 -- 8
		is_unique,                   -- 9
		is_indexed,                  -- 10
		max_length,                  -- 11
		is_computed,                 -- 12
		COALESCE(computed_expr, ''), -- 13
		default_v_bool,              -- 14
		default_v_int,               -- 15
		default_v_real,              -- 16
		default_v_text,              -- 17
		default_v_datetime,          -- 18
		created_at,                  -- 19
		updated_at                   -- 20
	FROM eav_attributes
	WHERE reference_id = ? AND deleted_at IS NULL;` // 1

	var attr db.EAVAttribute
	err := s.QueryRow(sqlSelect,
		refID, // 1
	).Scan(
		&attr.ID,               // 1
		&attr.ReferenceID,      // 2
		&attr.EntityTypeID,     // 3
		&attr.MachineName,      // 4
		&attr.Label,            // 5
		&attr.HelpText,         // 6
		&attr.PrimitiveKind,    // 7
		&attr.IsRequired,       // 8
		&attr.IsUnique,         // 9
		&attr.IsIndexed,        // 10
		&attr.MaxLength,        // 11
		&attr.IsComputed,       // 12
		&attr.ComputedExpr,     // 13
		&attr.DefaultVBool,     // 14
		&attr.DefaultVInt,      // 15
		&attr.DefaultVReal,     // 16
		&attr.DefaultVText,     // 17
		&attr.DefaultVDatetime, // 18
		&attr.CreatedAt,        // 19
		&attr.UpdatedAt,        // 20
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, db.ErrNotFound
		}
		return nil, err
	}
	return &attr, nil
}

// ListEAVAttributesByEntityTypeID returns all active attributes for an entity type,
// ordered by machine_name for predictable iteration.
func (s *SQLite) ListEAVAttributesByEntityTypeID(entityTypeID int64) ([]db.EAVAttribute, error) {
	const sqlSelect = `SELECT
		id,                          -- 1
		reference_id,                -- 2
		entity_type_id,              -- 3
		machine_name,                -- 4
		label,                       -- 5
		COALESCE(help_text, ''),     -- 6
		primitive_kind,              -- 7
		is_required,                 -- 8
		is_unique,                   -- 9
		is_indexed,                  -- 10
		max_length,                  -- 11
		is_computed,                 -- 12
		COALESCE(computed_expr, ''), -- 13
		default_v_bool,              -- 14
		default_v_int,               -- 15
		default_v_real,              -- 16
		default_v_text,              -- 17
		default_v_datetime,          -- 18
		created_at,                  -- 19
		updated_at                   -- 20
	FROM eav_attributes
	WHERE entity_type_id = ? AND deleted_at IS NULL
	ORDER BY machine_name ASC;` // 1

	rows, err := s.Query(sqlSelect,
		entityTypeID, // 1
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.EAVAttribute
	for rows.Next() {
		var attr db.EAVAttribute
		err := rows.Scan(
			&attr.ID,               // 1
			&attr.ReferenceID,      // 2
			&attr.EntityTypeID,     // 3
			&attr.MachineName,      // 4
			&attr.Label,            // 5
			&attr.HelpText,         // 6
			&attr.PrimitiveKind,    // 7
			&attr.IsRequired,       // 8
			&attr.IsUnique,         // 9
			&attr.IsIndexed,        // 10
			&attr.MaxLength,        // 11
			&attr.IsComputed,       // 12
			&attr.ComputedExpr,     // 13
			&attr.DefaultVBool,     // 14
			&attr.DefaultVInt,      // 15
			&attr.DefaultVReal,     // 16
			&attr.DefaultVText,     // 17
			&attr.DefaultVDatetime, // 18
			&attr.CreatedAt,        // 19
			&attr.UpdatedAt,        // 20
		)
		if err != nil {
			return nil, err
		}
		list = append(list, attr)
	}
	return list, rows.Err()
}

// SoftDeleteEAVAttribute marks an attribute as deleted.
func (s *SQLite) SoftDeleteEAVAttribute(id int64) error {
	const sqlUpdate = `UPDATE eav_attributes
	SET deleted_at = CURRENT_TIMESTAMP
	WHERE id = ? AND deleted_at IS NULL;` // 1

	err := s.Exec(sqlUpdate, id)
	if err != nil {
		return err
	}
	return nil
}

// ====================================================================
// Record Operations
// ====================================================================

// CreateEAVRecord creates a new record with rev=1 and opaque reference_id.
func (s *SQLite) CreateEAVRecord(entityTypeID int64) (*db.EAVRecord, error) {
	refID := utils.NewOpaqueID()
	const sqlInsert = `INSERT INTO eav_records (
		reference_id,   -- 1
		entity_type_id, -- 2
		rev,
		created_at,
		updated_at
	) VALUES (
		?,                 -- 1
		?,                 -- 2
		1,                 -- rev
		CURRENT_TIMESTAMP, -- created_at
		CURRENT_TIMESTAMP  -- updated_at
	) RETURNING
		id,             -- 1
		reference_id,   -- 2
		entity_type_id, -- 3
		rev,            -- 4
		created_at,     -- 5
		updated_at      -- 6
	;`

	var rec db.EAVRecord
	err := s.QueryRowRW(sqlInsert,
		refID,        // 1
		entityTypeID, // 2
	).Scan(
		&rec.ID,           // 1
		&rec.ReferenceID,  // 2
		&rec.EntityTypeID, // 3
		&rec.Rev,          // 4
		&rec.CreatedAt,    // 5
		&rec.UpdatedAt,    // 6
	)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// UpdateEAVRecordStatus updates the status of a record (e.g., from 'draft' to 'active')
// and increments rev for optimistic locking
func (s *SQLite) UpdateEAVRecordStatus(id int64, currentRev int, status string) error {
	const query = `UPDATE eav_records
	SET
		status = ?,     -- 1
		rev = ?,        -- 2
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?        -- 3
	AND rev = ?         -- 4
	AND deleted_at IS NULL;`
	result, err := s.rw.Exec(
		query,
		status,       // 1
		currentRev+1, // 2
		id,           // 3
		currentRev,   // 4
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return db.ErrConflict // Rev mismatch or record not found
	}

	return nil
}

// GetEAVRecordByID retrieves a record by internal ID.
func (s *SQLite) GetEAVRecordByID(id int64) (*db.EAVRecord, error) {
	const sqlSelect = `SELECT
		id,             -- 1
		reference_id,   -- 2
		entity_type_id, -- 3
		status,         -- 4
		rev,            -- 5
		created_at,     -- 6
		updated_at      -- 7
	FROM eav_records
	WHERE id = ? AND deleted_at IS NULL;` // 1

	var rec db.EAVRecord
	err := s.QueryRow(sqlSelect,
		id, // 1
	).Scan(
		&rec.ID,           // 1
		&rec.ReferenceID,  // 2
		&rec.EntityTypeID, // 3
		&rec.Status,       // 4
		&rec.Rev,          // 5
		&rec.CreatedAt,    // 6
		&rec.UpdatedAt,    // 7
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) { // Changed from `err == sql.ErrNoRows` to `errors.Is(err, db.ErrNoRows)` to match existing pattern
			return nil, db.ErrNotFound
		}
		return nil, err
	}
	return &rec, nil
}

// GetEAVRecordByRefID retrieves a record by opaque reference_id.
func (s *SQLite) GetEAVRecordByRefID(refID string) (*db.EAVRecord, error) {
	const sqlSelect = `SELECT
		id,             -- 1
		reference_id,   -- 2
		entity_type_id, -- 3
		status,         -- 4
		rev,            -- 5
		created_at,     -- 6
		updated_at      -- 7
	FROM eav_records
	WHERE reference_id = ? AND deleted_at IS NULL;` // 1

	var rec db.EAVRecord
	err := s.QueryRow(sqlSelect,
		refID, // 1
	).Scan(
		&rec.ID,           // 1
		&rec.ReferenceID,  // 2
		&rec.EntityTypeID, // 3
		&rec.Status,       // 4
		&rec.Rev,          // 5
		&rec.CreatedAt,    // 6
		&rec.UpdatedAt,    // 7
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) { // Changed from `err == sql.ErrNoRows` to `errors.Is(err, db.ErrNoRows)` to match existing pattern
			return nil, db.ErrNotFound
		}
		return nil, err
	}
	return &rec, nil
}

// ListEAVRecordsByEntityTypeID returns paginated records for an entity type,
// ordered by updated_at DESC, id DESC. Returns records and total count.
func (s *SQLite) ListEAVRecordsByEntityTypeID(entityTypeID int64, limit, offset int) ([]db.EAVRecord, int, error) {
	// Get total count
	const sqlCount = `SELECT COUNT(*)
	FROM eav_records
	WHERE entity_type_id = ? AND deleted_at IS NULL;` // 1

	var total int
	err := s.QueryRow(sqlCount, entityTypeID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated records
	const sqlSelect = `SELECT
		id,             -- 1
		reference_id,   -- 2
		entity_type_id, -- 3
		status,         -- 4
		rev,            -- 5
		created_at,     -- 6
		updated_at      -- 7
	FROM eav_records
	WHERE entity_type_id = ? AND deleted_at IS NULL
	ORDER BY created_at DESC, id DESC
	LIMIT ? OFFSET ?;` // 1, 2, 3

	rows, err := s.Query(sqlSelect,
		entityTypeID, // 1
		limit,        // 2
		offset,       // 3
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []db.EAVRecord
	for rows.Next() {
		var rec db.EAVRecord
		err := rows.Scan(
			&rec.ID,           // 1
			&rec.ReferenceID,  // 2
			&rec.EntityTypeID, // 3
			&rec.Status,       // 4
			&rec.Rev,          // 5
			&rec.CreatedAt,    // 6
			&rec.UpdatedAt,    // 7
		)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, rec)
	}
	return list, total, rows.Err()
}

// UpdateEAVRecordRev implements optimistic locking by incrementing rev only if currentRev matches.
// The validation is enforced by a SQLite trigger (trg_eav_records_update_rev).
// Returns the new rev on success, or db.ErrConflict if the rev doesn't match or trigger fails.
func (s *SQLite) UpdateEAVRecordRev(id int64, currentRev int) (int, error) {
	const sqlUpdate = `UPDATE eav_records
	SET
		rev = rev + 1,
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ? AND rev = ? AND deleted_at IS NULL
	RETURNING rev;` // 1, 2

	var newRev int
	err := s.QueryRowRW(sqlUpdate,
		id,         // 1
		currentRev, // 2
	).Scan(&newRev)
	if err != nil {
		// db.ErrNoRows means WHERE clause didn't match (wrong rev or deleted)
		if errors.Is(err, db.ErrNoRows) {
			return 0, db.ErrConflict
		}
		// Any other error (including trigger RAISE(ABORT)) is also a conflict
		return 0, fmt.Errorf("update record rev: %w", err)
	}
	return newRev, nil
}

// SoftDeleteEAVRecord marks a record as deleted.
func (s *SQLite) SoftDeleteEAVRecord(id int64) error {
	const sqlUpdate = `UPDATE eav_records
	SET deleted_at = CURRENT_TIMESTAMP
	WHERE id = ? AND deleted_at IS NULL;` // 1

	err := s.Exec(sqlUpdate, id)
	if err != nil {
		return err
	}
	return nil
}

// ====================================================================
// Value Operations
// ====================================================================

// UpsertEAVValue inserts or updates a value for a (record, attribute) pair.
// Validates that exactly one value is non-nil and matches the attribute's primitive_kind.
func (s *SQLite) UpsertEAVValue(
	recordID, attributeID int64,
	vBool *bool, vInt *int64, vReal *float64, vText, vDatetime *string,
) error {
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
		return fmt.Errorf("%w: exactly one value must be set", db.ErrInvalidValue)
	}

	// Fetch attribute to validate type
	attr, err := s.GetEAVAttributeByID(attributeID)
	if err != nil {
		return err
	}

	// Validate type matches
	switch attr.PrimitiveKind {
	case "BOOL":
		if vBool == nil {
			return fmt.Errorf("%w: attribute %s requires BOOL value", db.ErrInvalidValue, attr.MachineName)
		}
	case "INT":
		if vInt == nil {
			return fmt.Errorf("%w: attribute %s requires INT value", db.ErrInvalidValue, attr.MachineName)
		}
	case "REAL":
		if vReal == nil {
			return fmt.Errorf("%w: attribute %s requires REAL value", db.ErrInvalidValue, attr.MachineName)
		}
	case "TEXT":
		if vText == nil {
			return fmt.Errorf("%w: attribute %s requires TEXT value", db.ErrInvalidValue, attr.MachineName)
		}
	case "DATETIME":
		if vDatetime == nil {
			return fmt.Errorf("%w: attribute %s requires DATETIME value", db.ErrInvalidValue, attr.MachineName)
		}
	default:
		return fmt.Errorf("%w: unknown primitive_kind %s", db.ErrInvalidValue, attr.PrimitiveKind)
	}

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
		?,                 -- 1
		?,                 -- 2
		?,                 -- 3
		?,                 -- 4
		?,                 -- 5
		?,                 -- 6
		?,                 -- 7
		CURRENT_TIMESTAMP  -- updated_at
	) ON CONFLICT(record_id, attribute_id) DO UPDATE SET
		v_bool = excluded.v_bool,
		v_int = excluded.v_int,
		v_real = excluded.v_real,
		v_text = excluded.v_text,
		v_datetime = excluded.v_datetime,
		updated_at = CURRENT_TIMESTAMP;`

	return s.Exec(sqlUpsert,
		recordID,    // 1
		attributeID, // 2
		vBool,       // 3
		vInt,        // 4
		vReal,       // 5
		vText,       // 6
		vDatetime,   // 7
	)
}

// DeleteEAVValue removes a value for a (record, attribute) pair.
func (s *SQLite) DeleteEAVValue(recordID, attributeID int64) error {
	const sqlDelete = `DELETE FROM eav_values
	WHERE record_id = ? AND attribute_id = ?;` // 1, 2

	return s.Exec(sqlDelete,
		recordID,    // 1
		attributeID, // 2
	)
}

// GetEAVValuesByRecordID retrieves all values for a record in a single query,
// excluding values for soft-deleted attributes.
func (s *SQLite) GetEAVValuesByRecordID(recordID int64) ([]db.EAVValue, error) {
	const sqlSelect = `SELECT
		v.record_id,     -- 1
		v.attribute_id,  -- 2
		v.v_bool,        -- 3
		v.v_int,         -- 4
		v.v_real,        -- 5
		v.v_text,        -- 6
		v.v_datetime,    -- 7
		v.updated_at     -- 8
	FROM eav_values v
	INNER JOIN eav_attributes a ON v.attribute_id = a.id
	WHERE v.record_id = ? AND a.deleted_at IS NULL;` // 1

	rows, err := s.Query(sqlSelect,
		recordID, // 1
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.EAVValue
	for rows.Next() {
		var val db.EAVValue
		err := rows.Scan(
			&val.RecordID,    // 1
			&val.AttributeID, // 2
			&val.VBool,       // 3
			&val.VInt,        // 4
			&val.VReal,       // 5
			&val.VText,       // 6
			&val.VDatetime,   // 7
			&val.UpdatedAt,   // 8
		)
		if err != nil {
			return nil, err
		}
		list = append(list, val)
	}
	return list, rows.Err()
}

// CountEAVRecords returns the total number of non-deleted records for an entity type.
func (s *SQLite) CountEAVRecords(entityTypeID int64) (int, error) {
	const q = `SELECT COUNT(*) FROM eav_records WHERE entity_type_id = ? AND deleted_at IS NULL`
	var n int
	err := s.QueryRow(q, entityTypeID).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// CountEAVRecordsWhere counts non-deleted records whose attribute value matches.
// The primitiveKind determines which v_* column is queried.
func (s *SQLite) CountEAVRecordsWhere(entityTypeID, attributeID int64, primitiveKind string, value any) (int, error) {
	var q string
	switch primitiveKind {
	case "BOOL":
		q = `SELECT COUNT(*) FROM eav_records r
			JOIN eav_values v ON v.record_id = r.id
			WHERE r.entity_type_id = ? AND v.attribute_id = ? AND r.deleted_at IS NULL AND v.v_bool = ?`
	case "INT":
		q = `SELECT COUNT(*) FROM eav_records r
			JOIN eav_values v ON v.record_id = r.id
			WHERE r.entity_type_id = ? AND v.attribute_id = ? AND r.deleted_at IS NULL AND v.v_int = ?`
	case "REAL":
		q = `SELECT COUNT(*) FROM eav_records r
			JOIN eav_values v ON v.record_id = r.id
			WHERE r.entity_type_id = ? AND v.attribute_id = ? AND r.deleted_at IS NULL AND v.v_real = ?`
	case "TEXT":
		q = `SELECT COUNT(*) FROM eav_records r
			JOIN eav_values v ON v.record_id = r.id
			WHERE r.entity_type_id = ? AND v.attribute_id = ? AND r.deleted_at IS NULL AND v.v_text = ?`
	case "DATETIME":
		q = `SELECT COUNT(*) FROM eav_records r
			JOIN eav_values v ON v.record_id = r.id
			WHERE r.entity_type_id = ? AND v.attribute_id = ? AND r.deleted_at IS NULL AND v.v_datetime = ?`
	default:
		return 0, fmt.Errorf("%w: unsupported primitive_kind %q", db.ErrInvalidValue, primitiveKind)
	}
	var n int
	err := s.QueryRow(q, entityTypeID, attributeID, value).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ListEAVRecordsByAttributeValue returns non-deleted records (up to 200) whose
// TEXT attribute value matches the given string, ordered by created_at DESC.
func (s *SQLite) ListEAVRecordsByAttributeValue(entityTypeID, attributeID int64, value string) ([]db.EAVRecord, error) {
	const q = `SELECT r.id, r.reference_id, r.entity_type_id, r.status, r.rev, r.created_at, r.updated_at
		FROM eav_records r
		JOIN eav_values v ON v.record_id = r.id
		WHERE r.entity_type_id = ? AND v.attribute_id = ? AND v.v_text = ? AND r.deleted_at IS NULL
		ORDER BY r.created_at DESC
		LIMIT 200`
	rows, err := s.Query(q, entityTypeID, attributeID, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []db.EAVRecord
	for rows.Next() {
		var r db.EAVRecord
		if err := rows.Scan(&r.ID, &r.ReferenceID, &r.EntityTypeID, &r.Status, &r.Rev, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
