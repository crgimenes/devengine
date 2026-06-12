package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/utils"
)

// CreateForm creates a new form.
func (s *SQLite) CreateForm(machineName, label, description string, eavEntityTypeID *int64) (*db.Form, error) {
	refID := utils.NewOpaqueID()
	const sqlInsert = `INSERT INTO forms (
		reference_id,       -- 1
		machine_name,       -- 2
		label,              -- 3
		description,        -- 4
		eav_entity_type_id, -- 5
		hide_submit_button,
		hide_cancel_button,
		hide_title,
		show_system_info
	) VALUES (
		?, -- 1
		?, -- 2
		?, -- 3
		?, -- 4
		?, -- 5
		0, -- hide_submit_button
		0, -- hide_cancel_button
		0, -- hide_title
		0  -- show_system_info
	)
	RETURNING
		id,                 -- 1
		reference_id,       -- 2
		machine_name,       -- 3
		label,              -- 4
		description,        -- 5
		eav_entity_type_id, -- 6
		hide_submit_button, -- 7
		hide_cancel_button, -- 8
		hide_title,         -- 9
		show_system_info,   -- 10
		created_at,         -- 11
		updated_at          -- 12
	`

	var f db.Form
	err := s.QueryRowRW(
		sqlInsert,
		refID,           // 1
		machineName,     // 2
		label,           // 3
		description,     // 4
		eavEntityTypeID, // 5
	).Scan(
		&f.ID,               // 1
		&f.ReferenceID,      // 2
		&f.MachineName,      // 3
		&f.Label,            // 4
		&f.Description,      // 5
		&f.EAVEntityTypeID,  // 6
		&f.HideSubmitButton, // 7
		&f.HideCancelButton, // 8
		&f.HideTitle,        // 9
		&f.ShowSystemInfo,   // 10
		&f.CreatedAt,        // 11
		&f.UpdatedAt,        // 12
	)
	if err != nil {
		return nil, fmt.Errorf("create form: %w", err)
	}
	return &f, nil
}

// GetFormByRefID retrieves a form by reference_id.
func (s *SQLite) GetFormByRefID(refID string) (*db.Form, error) {
	const q = `SELECT
		id,                 -- 1
		reference_id,       -- 2
		machine_name,       -- 3
		label,              -- 4
		description,        -- 5
		eav_entity_type_id, -- 6
		hide_submit_button, -- 7
		hide_cancel_button, -- 8
		hide_title,         -- 9
		show_system_info,   -- 10
		menu_id,            -- 11
		is_search,          -- 12
		created_at,         -- 13
		updated_at,         -- 14
		deleted_at          -- 15
	FROM forms
	WHERE reference_id = ? -- 1
	AND deleted_at IS NULL`

	var f db.Form
	err := s.QueryRow(
		q,
		refID, // 1
	).Scan(
		&f.ID,               // 1
		&f.ReferenceID,      // 2
		&f.MachineName,      // 3
		&f.Label,            // 4
		&f.Description,      // 5
		&f.EAVEntityTypeID,  // 6
		&f.HideSubmitButton, // 7
		&f.HideCancelButton, // 8
		&f.HideTitle,        // 9
		&f.ShowSystemInfo,   // 10
		&f.MenuID,           // 11
		&f.IsSearch,         // 12
		&f.CreatedAt,        // 13
		&f.UpdatedAt,        // 14
		&f.DeletedAt,        // 15
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("form not found: %s", refID)
		}
		return nil, fmt.Errorf("get form by ref id: %w", err)
	}
	return &f, nil
}

// GetFormByMachineName retrieves a form by machine_name.
// Returns nil, nil if not found (soft not-found to allow fallback logic).
func (s *SQLite) GetFormByMachineName(machineName string) (*db.Form, error) {
	const q = `SELECT
		id,                 -- 1
		reference_id,       -- 2
		machine_name,       -- 3
		label,              -- 4
		description,        -- 5
		eav_entity_type_id, -- 6
		hide_submit_button, -- 7
		hide_cancel_button, -- 8
		hide_title,         -- 9
		show_system_info,   -- 10
		menu_id,            -- 11
		is_search,          -- 12
		created_at,         -- 13
		updated_at,         -- 14
		deleted_at          -- 15
	FROM forms
	WHERE machine_name = ? -- 1
	AND deleted_at IS NULL`

	var f db.Form
	err := s.QueryRow(
		q,
		machineName, // 1
	).Scan(
		&f.ID,               // 1
		&f.ReferenceID,      // 2
		&f.MachineName,      // 3
		&f.Label,            // 4
		&f.Description,      // 5
		&f.EAVEntityTypeID,  // 6
		&f.HideSubmitButton, // 7
		&f.HideCancelButton, // 8
		&f.HideTitle,        // 9
		&f.ShowSystemInfo,   // 10
		&f.MenuID,           // 11
		&f.IsSearch,         // 12
		&f.CreatedAt,        // 13
		&f.UpdatedAt,        // 14
		&f.DeletedAt,        // 15
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found is OK, caller will use fallback
		}
		return nil, fmt.Errorf("get form by machine name: %w", err)
	}
	return &f, nil
}

// ListForms returns all non-deleted forms.
func (s *SQLite) ListForms() ([]db.Form, error) {
	const q = `SELECT
		id,                 -- 1
		reference_id,       -- 2
		machine_name,       -- 3
		label,              -- 4
		description,        -- 5
		eav_entity_type_id, -- 6
		hide_submit_button, -- 7
		hide_cancel_button, -- 8
		hide_title,         -- 9
		show_system_info,   -- 10
		menu_id,            -- 11
		is_search,          -- 12
		created_at,         -- 13
		updated_at          -- 14
	FROM forms
	WHERE deleted_at IS NULL
	ORDER BY label`

	rows, err := s.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list forms: %w", err)
	}
	defer rows.Close()

	var forms []db.Form
	for rows.Next() {
		var f db.Form
		if err := rows.Scan(
			&f.ID,               // 1
			&f.ReferenceID,      // 2
			&f.MachineName,      // 3
			&f.Label,            // 4
			&f.Description,      // 5
			&f.EAVEntityTypeID,  // 6
			&f.HideSubmitButton, // 7
			&f.HideCancelButton, // 8
			&f.HideTitle,        // 9
			&f.ShowSystemInfo,   // 10
			&f.MenuID,           // 11
			&f.IsSearch,         // 12
			&f.CreatedAt,        // 13
			&f.UpdatedAt,        // 14
		); err != nil {
			return nil, fmt.Errorf("scan form: %w", err)
		}
		forms = append(forms, f)
	}
	return forms, rows.Err()
}

// UpdateForm updates a form's basic info.
func (s *SQLite) UpdateForm(id int64, machineName, label, description string, eavEntityTypeID *int64, hideSubmitButton, hideCancelButton, hideTitle, showSystemInfo bool, menuID *int64, isSearch bool) error {
	const q = `UPDATE forms
	SET
		machine_name = ?,       -- 1
		label = ?,              -- 2
		description = ?,        -- 3
		eav_entity_type_id = ?, -- 4
		hide_submit_button = ?, -- 5
		hide_cancel_button = ?, -- 6
		hide_title = ?,         -- 7
		show_system_info = ?,   -- 8
		menu_id = ?,            -- 9
		is_search = ?,          -- 10
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?                -- 11
	AND deleted_at IS NULL`
	return s.Exec(
		q,
		machineName,      // 1
		label,            // 2
		description,      // 3
		eavEntityTypeID,  // 4
		hideSubmitButton, // 5
		hideCancelButton, // 6
		hideTitle,        // 7
		showSystemInfo,   // 8
		menuID,           // 9
		isSearch,         // 10
		id,               // 11
	)
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
	zOrder, colSpan int,
	uiKind, uiMetaJSON string,
	eavAttributeID *int64,
	isUIOnly, isReadonly bool,
) (*db.FormElement, error) {
	refID := utils.NewOpaqueID()
	// Default col_span to 12 if not set
	if colSpan < 1 || colSpan > 12 {
		colSpan = 12
	}
	const sqlInsert = `INSERT INTO form_elements (
		reference_id,       -- 1
		form_id,            -- 2
		parent_id,          -- 3
		machine_name,       -- 4
		element_kind,       -- 5
		label,              -- 6
		help_text,          -- 7
		z_order,            -- 8
		col_span,           -- 9
		alignment,
		ui_kind,            -- 10
		ui_meta_json,       -- 11
		eav_attribute_id,   -- 12
		is_ui_only,         -- 13
		is_readonly,        -- 14
		hide_label,
		hide_help_text,
		button_filo_code,
		button_run_save,
		button_js_code,
		button_style,
		button_confirm_msg
	) VALUES (
		?,         -- 1
		?,         -- 2
		?,         -- 3
		?,         -- 4
		?,         -- 5
		?,         -- 6
		?,         -- 7
		?,         -- 8
		?,         -- 9
		'left',    -- alignment
		?,         -- 10
		?,         -- 11
		?,         -- 12
		?,         -- 13
		?,         -- 14
		0,         -- hide_label
		0,         -- hide_help_text
		'',        -- button_filo_code
		0,         -- button_run_save
		'',        -- button_js_code
		'primary', -- button_style
		''         -- button_confirm_msg
	)
	RETURNING
		id,                 -- 1
		reference_id,       -- 2
		form_id,            -- 3
		parent_id,          -- 4
		machine_name,       -- 5
		element_kind,       -- 6
		label,              -- 7
		help_text,          -- 8
		z_order,            -- 9
		col_span,           -- 10
		alignment,          -- 11
		ui_kind,            -- 12
		ui_meta_json,       -- 13
		eav_attribute_id,   -- 14
		is_ui_only,         -- 15
		is_readonly,        -- 16
		hide_label,         -- 17
		hide_help_text,     -- 18
		button_filo_code,   -- 19
		button_run_save,    -- 20
		button_js_code,     -- 21
		button_style,       -- 22
		button_confirm_msg, -- 23
		created_at,         -- 24
		updated_at          -- 25
	`

	var e db.FormElement
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

	err := s.QueryRowRW(
		sqlInsert,
		refID,                 // 1
		formID,                // 2
		parentIDVal,           // 3
		machineName,           // 4
		elementKind,           // 5
		label,                 // 6
		helpText,              // 7
		zOrder,                // 8
		colSpan,               // 9
		uiKind,                // 10
		uiMetaJSON,            // 11
		eavAttrIDVal,          // 12
		boolToInt(isUIOnly),   // 13
		boolToInt(isReadonly), // 14
	).Scan(
		&e.ID,               // 1
		&e.ReferenceID,      // 2
		&e.FormID,           // 3
		&scanParentID,       // 4
		&e.MachineName,      // 5
		&e.ElementKind,      // 6
		&e.Label,            // 7
		&e.HelpText,         // 8
		&e.ZOrder,           // 9
		&e.ColSpan,          // 10
		&e.Alignment,        // 11
		&e.UIKind,           // 12
		&e.UIMetaJSON,       // 13
		&scanEAVAttrID,      // 14
		&e.IsUIOnly,         // 15
		&e.IsReadonly,       // 16
		&e.HideLabel,        // 17
		&e.HideHelpText,     // 18
		&e.ButtonFiloCode,   // 19
		&e.ButtonRunSave,    // 20
		&e.ButtonJSCode,     // 21
		&e.ButtonStyle,      // 22
		&e.ButtonConfirmMsg, // 23
		&e.CreatedAt,        // 24
		&e.UpdatedAt,        // 25
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
func (s *SQLite) ListFormElements(formID int64) ([]db.FormElement, error) {
	const q = `SELECT
		id,                 -- 1
		reference_id,       -- 2
		form_id,            -- 3
		parent_id,          -- 4
		machine_name,       -- 5
		element_kind,       -- 6
		label,              -- 7
		help_text,          -- 8
		z_order,            -- 9
		col_span,           -- 10
		alignment,          -- 11
		ui_kind,            -- 12
		ui_meta_json,       -- 13
		eav_attribute_id,   -- 14
		is_ui_only,         -- 15
		is_readonly,        -- 16
		hide_label,         -- 17
		hide_help_text,     -- 18
		button_filo_code,   -- 19
		button_run_save,    -- 20
		button_js_code,     -- 21
		button_style,       -- 22
		button_confirm_msg, -- 23
		COALESCE(validate_expr, ''), -- 24
		created_at,         -- 25
		updated_at          -- 26
	FROM form_elements
	WHERE form_id = ?       -- 1
	AND deleted_at IS NULL
	ORDER BY COALESCE(parent_id, 0), z_order, id`

	rows, err := s.Query(
		q,
		formID, // 1
	)
	if err != nil {
		return nil, fmt.Errorf("list form elements: %w", err)
	}
	defer rows.Close()

	var elements []db.FormElement
	for rows.Next() {
		var e db.FormElement
		var parentID sql.NullInt64
		var eavAttrID sql.NullInt64

		if err := rows.Scan(
			&e.ID,               // 1
			&e.ReferenceID,      // 2
			&e.FormID,           // 3
			&parentID,           // 4
			&e.MachineName,      // 5
			&e.ElementKind,      // 6
			&e.Label,            // 7
			&e.HelpText,         // 8
			&e.ZOrder,           // 9
			&e.ColSpan,          // 10
			&e.Alignment,        // 11
			&e.UIKind,           // 12
			&e.UIMetaJSON,       // 13
			&eavAttrID,          // 14
			&e.IsUIOnly,         // 15
			&e.IsReadonly,       // 16
			&e.HideLabel,        // 17
			&e.HideHelpText,     // 18
			&e.ButtonFiloCode,   // 19
			&e.ButtonRunSave,    // 20
			&e.ButtonJSCode,     // 21
			&e.ButtonStyle,      // 22
			&e.ButtonConfirmMsg, // 23
			&e.ValidateExpr,     // 24
			&e.CreatedAt,        // 25
			&e.UpdatedAt,        // 26
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
func (s *SQLite) GetFormElementByRefID(refID string) (*db.FormElement, error) {
	const q = `SELECT
		id,                 -- 1
		reference_id,       -- 2
		form_id,            -- 3
		parent_id,          -- 4
		machine_name,       -- 5
		element_kind,       -- 6
		label,              -- 7
		help_text,          -- 8
		z_order,            -- 9
		col_span,           -- 10
		alignment,          -- 11
		ui_kind,            -- 12
		ui_meta_json,       -- 13
		eav_attribute_id,   -- 14
		is_ui_only,         -- 15
		is_readonly,        -- 16
		hide_label,         -- 17
		hide_help_text,     -- 18
		button_filo_code,   -- 19
		button_run_save,    -- 20
		button_js_code,     -- 21
		button_style,       -- 22
		button_confirm_msg, -- 23
		COALESCE(validate_expr, ''), -- 24
		created_at,         -- 25
		updated_at          -- 26
	FROM form_elements
	WHERE reference_id = ? -- 1
	AND deleted_at IS NULL`

	var e db.FormElement
	var parentID sql.NullInt64
	var eavAttrID sql.NullInt64

	err := s.QueryRow(
		q,
		refID, // 1
	).Scan(
		&e.ID,               // 1
		&e.ReferenceID,      // 2
		&e.FormID,           // 3
		&parentID,           // 4
		&e.MachineName,      // 5
		&e.ElementKind,      // 6
		&e.Label,            // 7
		&e.HelpText,         // 8
		&e.ZOrder,           // 9
		&e.ColSpan,          // 10
		&e.Alignment,        // 11
		&e.UIKind,           // 12
		&e.UIMetaJSON,       // 13
		&eavAttrID,          // 14
		&e.IsUIOnly,         // 15
		&e.IsReadonly,       // 16
		&e.HideLabel,        // 17
		&e.HideHelpText,     // 18
		&e.ButtonFiloCode,   // 19
		&e.ButtonRunSave,    // 20
		&e.ButtonJSCode,     // 21
		&e.ButtonStyle,      // 22
		&e.ButtonConfirmMsg, // 23
		&e.ValidateExpr,     // 24
		&e.CreatedAt,        // 25
		&e.UpdatedAt,        // 26
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

// UpdateFormElement updates an existing form element.
func (s *SQLite) UpdateFormElement(
	id int64,
	parentID *int64,
	machineName, elementKind, label, helpText string,
	zOrder, colSpan int,
	alignment string,
	uiKind, uiMetaJSON string,
	eavAttributeID *int64,
	isUIOnly, isReadonly, hideLabel, hideHelpText bool,
	validateExpr string,
	buttonFiloCode string, buttonRunSave bool, buttonJSCode, buttonStyle, buttonConfirmMsg string,
) error {
	// Default col_span to 12 if not set
	if colSpan < 1 || colSpan > 12 {
		colSpan = 12
	}
	// Default button style
	if buttonStyle == "" {
		buttonStyle = "primary"
	}
	const q = `UPDATE form_elements
	SET
		parent_id = ?,          -- 1
		machine_name = ?,       -- 2
		element_kind = ?,       -- 3
		label = ?,              -- 4
		help_text = ?,          -- 5
		z_order = ?,            -- 6
		col_span = ?,           -- 7
		alignment = ?,          -- 8
		ui_kind = ?,            -- 9
		ui_meta_json = ?,       -- 10
		eav_attribute_id = ?,   -- 11
		is_ui_only = ?,         -- 12
		is_readonly = ?,        -- 13
		hide_label = ?,         -- 14
		hide_help_text = ?,     -- 15
		validate_expr = ?,      -- 16
		button_filo_code = ?,   -- 17
		button_run_save = ?,    -- 18
		button_js_code = ?,     -- 19
		button_style = ?,       -- 20
		button_confirm_msg = ?, -- 21
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?                -- 22
	AND deleted_at IS NULL`

	var parentIDVal sql.NullInt64
	if parentID != nil {
		parentIDVal = sql.NullInt64{Int64: *parentID, Valid: true}
	}
	var eavAttrIDVal sql.NullInt64
	if eavAttributeID != nil {
		eavAttrIDVal = sql.NullInt64{Int64: *eavAttributeID, Valid: true}
	}

	return s.Exec(
		q,
		parentIDVal,              // 1
		machineName,              // 2
		elementKind,              // 3
		label,                    // 4
		helpText,                 // 5
		zOrder,                   // 6
		colSpan,                  // 7
		alignment,                // 8
		uiKind,                   // 9
		uiMetaJSON,               // 10
		eavAttrIDVal,             // 11
		boolToInt(isUIOnly),      // 12
		boolToInt(isReadonly),    // 13
		boolToInt(hideLabel),     // 14
		boolToInt(hideHelpText),  // 15
		validateExpr,             // 16
		buttonFiloCode,           // 17
		boolToInt(buttonRunSave), // 18
		buttonJSCode,             // 19
		buttonStyle,              // 20
		buttonConfirmMsg,         // 21
		id,                       // 22
	)
}

// ListGroupElements returns elements that can be parents (groups, accordions, cards, tabs).
func (s *SQLite) ListGroupElements(formID int64) ([]db.FormElement, error) {
	const q = `SELECT
		id,               -- 1
		reference_id,     -- 2
		form_id,          -- 3
		parent_id,        -- 4
		machine_name,     -- 5
		element_kind,     -- 6
		label,            -- 7
		help_text,        -- 8
		z_order,          -- 9
		col_span,         -- 10
		alignment,        -- 11
		ui_kind,          -- 12
		ui_meta_json,     -- 13
		eav_attribute_id, -- 14
		is_ui_only,       -- 15
		is_readonly,      -- 16
		hide_label,       -- 17
		hide_help_text,   -- 18
		created_at,       -- 19
		updated_at        -- 20
	FROM form_elements
	WHERE form_id = ?     -- 1
	AND deleted_at IS NULL
	AND element_kind IN ('group', 'accordion', 'card', 'tabs')
	ORDER BY z_order, id`

	rows, err := s.Query(
		q,
		formID, // 1
	)
	if err != nil {
		return nil, fmt.Errorf("list group elements: %w", err)
	}
	defer rows.Close()

	var elements []db.FormElement
	for rows.Next() {
		var e db.FormElement
		var parentID sql.NullInt64
		var eavAttrID sql.NullInt64

		if err := rows.Scan(
			&e.ID,           // 1
			&e.ReferenceID,  // 2
			&e.FormID,       // 3
			&parentID,       // 4
			&e.MachineName,  // 5
			&e.ElementKind,  // 6
			&e.Label,        // 7
			&e.HelpText,     // 8
			&e.ZOrder,       // 9
			&e.ColSpan,      // 10
			&e.Alignment,    // 11
			&e.UIKind,       // 12
			&e.UIMetaJSON,   // 13
			&eavAttrID,      // 14
			&e.IsUIOnly,     // 15
			&e.IsReadonly,   // 16
			&e.HideLabel,    // 17
			&e.HideHelpText, // 18
			&e.CreatedAt,    // 19
			&e.UpdatedAt,    // 20
		); err != nil {
			return nil, fmt.Errorf("scan group element: %w", err)
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

// boolToInt converts bool to SQLite integer.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// MoveElementUp swaps this element's z_order with the previous element (lower z_order, same parent).
func (s *SQLite) MoveElementUp(elementID int64) error {
	tx, err := s.BeginTransaction()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get current element
	var formID int64
	var currentZOrder int
	var parentID sql.NullInt64
	const qCurrent = `SELECT
		form_id,   -- 1
		z_order,   -- 2
		parent_id  -- 3
	FROM form_elements
	WHERE id = ?   -- 1
	AND deleted_at IS NULL`
	if err := tx.QueryRow(qCurrent, elementID).Scan(
		&formID,        // 1
		&currentZOrder, // 2
		&parentID,      // 3
	); err != nil {
		return fmt.Errorf("get current element: %w", err)
	}

	// Find previous element (same parent, lower z_order, max z_order less than current)
	var prevID int64
	var prevZOrder int
	var qPrev string
	var args []any
	if parentID.Valid {
		qPrev = `SELECT
			id,      -- 1
			z_order  -- 2
		FROM form_elements
		WHERE form_id = ?   -- 1
		AND parent_id = ?   -- 2
		AND z_order < ?     -- 3
		AND deleted_at IS NULL
		ORDER BY z_order DESC LIMIT 1`
		args = []any{formID, parentID.Int64, currentZOrder}
	} else {
		qPrev = `SELECT
			id,      -- 1
			z_order  -- 2
		FROM form_elements
		WHERE form_id = ?   -- 1
		AND parent_id IS NULL
		AND z_order < ?     -- 2
		AND deleted_at IS NULL
		ORDER BY z_order DESC LIMIT 1`
		args = []any{formID, currentZOrder}
	}
	if err := tx.QueryRow(qPrev, args...).Scan(
		&prevID,     // 1
		&prevZOrder, // 2
	); err != nil {
		if err == sql.ErrNoRows {
			return nil // Already at top
		}
		return fmt.Errorf("find previous element: %w", err)
	}

	// Swap z_order values
	const qUpdate = `UPDATE form_elements
	SET
		z_order = ?,  -- 1
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?      -- 2
	`
	if err := tx.Exec(qUpdate, prevZOrder, elementID); err != nil {
		return fmt.Errorf("update current z_order: %w", err)
	}
	if err := tx.Exec(qUpdate, currentZOrder, prevID); err != nil {
		return fmt.Errorf("update previous z_order: %w", err)
	}

	return tx.Commit()
}

// MoveElementDown swaps this element's z_order with the next element (higher z_order, same parent).
func (s *SQLite) MoveElementDown(elementID int64) error {
	tx, err := s.BeginTransaction()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get current element
	var formID int64
	var currentZOrder int
	var parentID sql.NullInt64
	const qCurrent = `SELECT
		form_id,   -- 1
		z_order,   -- 2
		parent_id  -- 3
	FROM form_elements
	WHERE id = ?   -- 1
	AND deleted_at IS NULL`
	if err := tx.QueryRow(qCurrent, elementID).Scan(
		&formID,        // 1
		&currentZOrder, // 2
		&parentID,      // 3
	); err != nil {
		return fmt.Errorf("get current element: %w", err)
	}

	// Find next element (same parent, higher z_order, min z_order greater than current)
	var nextID int64
	var nextZOrder int
	var qNext string
	var args []any
	if parentID.Valid {
		qNext = `SELECT
			id,      -- 1
			z_order  -- 2
		FROM form_elements
		WHERE form_id = ?   -- 1
		AND parent_id = ?   -- 2
		AND z_order > ?     -- 3
		AND deleted_at IS NULL
		ORDER BY z_order ASC LIMIT 1`
		args = []any{formID, parentID.Int64, currentZOrder}
	} else {
		qNext = `SELECT
			id,      -- 1
			z_order  -- 2
		FROM form_elements
		WHERE form_id = ?   -- 1
		AND parent_id IS NULL
		AND z_order > ?     -- 2
		AND deleted_at IS NULL
		ORDER BY z_order ASC LIMIT 1`
		args = []any{formID, currentZOrder}
	}
	if err := tx.QueryRow(qNext, args...).Scan(
		&nextID,     // 1
		&nextZOrder, // 2
	); err != nil {
		if err == sql.ErrNoRows {
			return nil // Already at bottom
		}
		return fmt.Errorf("find next element: %w", err)
	}

	// Swap z_order values
	const qUpdate = `UPDATE form_elements
	SET
		z_order = ?,  -- 1
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?      -- 2
	`
	if err := tx.Exec(qUpdate, nextZOrder, elementID); err != nil {
		return fmt.Errorf("update current z_order: %w", err)
	}
	if err := tx.Exec(qUpdate, currentZOrder, nextID); err != nil {
		return fmt.Errorf("update next z_order: %w", err)
	}

	return tx.Commit()
}
