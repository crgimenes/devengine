package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/crgimenes/devengine/utils"
)

// Form represents a form definition.
type Form struct {
	ID              int64
	ReferenceID     string
	MachineName     string
	Label           string
	Description     string
	EAVEntityTypeID *int64 // NULL if not bound to EAV
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// FormElement represents a UI element within a form (field, group, divider, etc.).
type FormElement struct {
	ID             int64
	ReferenceID    string
	FormID         int64
	ParentID       *int64 // NULL for root-level elements
	MachineName    string
	ElementKind    string // 'group', 'field', 'divider', etc.
	Label          string
	HelpText       string
	ZOrder         int
	UIKind         string // Plugin ID
	UIMetaJSON     string
	EAVAttributeID *int64 // NULL for UI-only elements
	IsUIOnly       bool
	IsReadonly     bool
	ValidateExpr   string
	ComputedExpr   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// CreateForm creates a new form.
func (s *SQLite) CreateForm(machineName, label, description string, eavEntityTypeID *int64) (*Form, error) {
	refID := utils.NewOpaqueID()
	const sqlInsert = `
		INSERT INTO forms (reference_id, machine_name, label, description, eav_entity_type_id)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, reference_id, machine_name, label, description, eav_entity_type_id, created_at, updated_at
	`

	var f Form
	err := s.QueryRowRW(sqlInsert, refID, machineName, label, description, eavEntityTypeID).Scan(
		&f.ID, &f.ReferenceID, &f.MachineName, &f.Label, &f.Description,
		&f.EAVEntityTypeID, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create form: %w", err)
	}
	return &f, nil
}

// GetFormByRefID retrieves a form by reference_id.
func (s *SQLite) GetFormByRefID(refID string) (*Form, error) {
	const q = `
		SELECT id, reference_id, machine_name, label, description, eav_entity_type_id,
		       created_at, updated_at, deleted_at
		FROM forms
		WHERE reference_id = ? AND deleted_at IS NULL
	`

	var f Form
	err := s.QueryRow(q, refID).Scan(
		&f.ID, &f.ReferenceID, &f.MachineName, &f.Label, &f.Description,
		&f.EAVEntityTypeID, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("form not found: %s", refID)
		}
		return nil, fmt.Errorf("get form by ref id: %w", err)
	}
	return &f, nil
}

// ListForms returns all non-deleted forms.
func (s *SQLite) ListForms() ([]Form, error) {
	const q = `
		SELECT id, reference_id, machine_name, label, description, eav_entity_type_id,
		       created_at, updated_at
		FROM forms
		WHERE deleted_at IS NULL
		ORDER BY label
	`

	rows, err := s.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list forms: %w", err)
	}
	defer rows.Close()

	var forms []Form
	for rows.Next() {
		var f Form
		if err := rows.Scan(
			&f.ID, &f.ReferenceID, &f.MachineName, &f.Label, &f.Description,
			&f.EAVEntityTypeID, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan form: %w", err)
		}
		forms = append(forms, f)
	}
	return forms, rows.Err()
}

// UpdateForm updates a form's basic info.
func (s *SQLite) UpdateForm(id int64, machineName, label, description string, eavEntityTypeID *int64) error {
	const q = `
		UPDATE forms
		SET machine_name = ?, label = ?, description = ?, eav_entity_type_id = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL
	`
	return s.Exec(q, machineName, label, description, eavEntityTypeID, id)
}

// SoftDeleteForm soft-deletes a form.
func (s *SQLite) SoftDeleteForm(id int64) error {
	const q = `UPDATE forms SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`
	return s.Exec(q, id)
}

// CreateFormElement creates a new form element.
func (s *SQLite) CreateFormElement(
	formID int64,
	parentID *int64,
	machineName, elementKind, label, helpText string,
	zOrder int,
	uiKind, uiMetaJSON string,
	eavAttributeID *int64,
	isUIOnly, isReadonly bool,
) (*FormElement, error) {
	refID := utils.NewOpaqueID()
	const sqlInsert = `
		INSERT INTO form_elements (
			reference_id, form_id, parent_id, machine_name, element_kind,
			label, help_text, z_order, ui_kind, ui_meta_json,
			eav_attribute_id, is_ui_only, is_readonly
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, reference_id, form_id, parent_id, machine_name, element_kind,
		          label, help_text, z_order, ui_kind, ui_meta_json,
		          eav_attribute_id, is_ui_only, is_readonly, created_at, updated_at
	`

	var e FormElement
	var parentIDVal sql.NullInt64
	if parentID != nil {
		parentIDVal = sql.NullInt64{Int64: *parentID, Valid: true}
	}
	var eavAttrIDVal sql.NullInt64
	if eavAttributeID != nil {
		eavAttrIDVal = sql.NullInt64{Int64: *eavAttributeID, Valid: true}
	}

	var scanParentID sql.NullInt64
	var scanEAVAttrID sql.NullInt64

	err := s.QueryRowRW(sqlInsert,
		refID, formID, parentIDVal, machineName, elementKind, label, helpText, zOrder,
		uiKind, uiMetaJSON, eavAttrIDVal, boolToInt(isUIOnly), boolToInt(isReadonly),
	).Scan(
		&e.ID, &e.ReferenceID, &e.FormID, &scanParentID, &e.MachineName, &e.ElementKind,
		&e.Label, &e.HelpText, &e.ZOrder, &e.UIKind, &e.UIMetaJSON,
		&scanEAVAttrID, &e.IsUIOnly, &e.IsReadonly, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create form element: %w", err)
	}

	if scanParentID.Valid {
		e.ParentID = &scanParentID.Int64
	}
	if scanEAVAttrID.Valid {
		e.EAVAttributeID = &scanEAVAttrID.Int64
	}

	return &e, nil
}

// ListFormElements returns all non-deleted elements for a form, ordered by z_order.
func (s *SQLite) ListFormElements(formID int64) ([]FormElement, error) {
	const q = `
		SELECT id, reference_id, form_id, parent_id, machine_name, element_kind,
		       label, help_text, z_order, ui_kind, ui_meta_json,
		       eav_attribute_id, is_ui_only, is_readonly, created_at, updated_at
		FROM form_elements
		WHERE form_id = ? AND deleted_at IS NULL
		ORDER BY COALESCE(parent_id, 0), z_order, id
	`

	rows, err := s.Query(q, formID)
	if err != nil {
		return nil, fmt.Errorf("list form elements: %w", err)
	}
	defer rows.Close()

	var elements []FormElement
	for rows.Next() {
		var e FormElement
		var parentID sql.NullInt64
		var eavAttrID sql.NullInt64

		if err := rows.Scan(
			&e.ID, &e.ReferenceID, &e.FormID, &parentID, &e.MachineName, &e.ElementKind,
			&e.Label, &e.HelpText, &e.ZOrder, &e.UIKind, &e.UIMetaJSON,
			&eavAttrID, &e.IsUIOnly, &e.IsReadonly, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan form element: %w", err)
		}

		if parentID.Valid {
			e.ParentID = &parentID.Int64
		}
		if eavAttrID.Valid {
			e.EAVAttributeID = &eavAttrID.Int64
		}

		elements = append(elements, e)
	}
	return elements, rows.Err()
}

// GetFormElementByRefID retrieves a form element by reference_id.
func (s *SQLite) GetFormElementByRefID(refID string) (*FormElement, error) {
	const q = `
		SELECT id, reference_id, form_id, parent_id, machine_name, element_kind,
		       label, help_text, z_order, ui_kind, ui_meta_json,
		       eav_attribute_id, is_ui_only, is_readonly, created_at, updated_at
		FROM form_elements
		WHERE reference_id = ? AND deleted_at IS NULL
	`

	var e FormElement
	var parentID sql.NullInt64
	var eavAttrID sql.NullInt64

	err := s.QueryRow(q, refID).Scan(
		&e.ID, &e.ReferenceID, &e.FormID, &parentID, &e.MachineName, &e.ElementKind,
		&e.Label, &e.HelpText, &e.ZOrder, &e.UIKind, &e.UIMetaJSON,
		&eavAttrID, &e.IsUIOnly, &e.IsReadonly, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("form element not found: %s", refID)
		}
		return nil, fmt.Errorf("get form element by ref id: %w", err)
	}

	if parentID.Valid {
		e.ParentID = &parentID.Int64
	}
	if eavAttrID.Valid {
		e.EAVAttributeID = &eavAttrID.Int64
	}

	return &e, nil
}

// DeleteFormElement permanently deletes a form element.
func (s *SQLite) DeleteFormElement(id int64) error {
	const q = `DELETE FROM form_elements WHERE id = ?`
	return s.Exec(q, id)
}

// boolToInt converts bool to SQLite integer.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
