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
	ID               int64
	ReferenceID      string
	MachineName      string
	Label            string
	Description      string
	EAVEntityTypeID  *int64 // NULL if not bound to EAV
	HideSubmitButton bool   // When true, form does not display submit button
	HideCancelButton bool   // When true, form does not display cancel button
	HideTitle        bool   // When true, form does not display title header
	ShowSystemInfo   bool   // When true, displays record ID and status in runtime
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// FormElement represents a UI element within a form (field, group, divider, etc.).
type FormElement struct {
	ID             int64
	ReferenceID    string
	FormID         int64
	ParentID       *int64 // NULL for root-level elements
	MachineName    string
	ElementKind    string // 'group', 'field', 'divider', 'button', etc.
	Label          string
	HelpText       string
	ZOrder         int
	ColSpan        int    // Bootstrap column span (1-12, default 12)
	Alignment      string // Horizontal alignment: 'left', 'center', 'right'
	UIKind         string // Plugin ID
	UIMetaJSON     string
	EAVAttributeID *int64 // NULL for UI-only elements
	IsUIOnly       bool
	IsReadonly     bool
	ValidateExpr   string
	ComputedExpr   string
	// Button-specific properties (only used when ElementKind = 'button')
	ButtonFiloCode   string // Server-side Filo script (never sent to client)
	ButtonRunSave    bool   // Execute save action in same transaction
	ButtonJSCode     string // Client-side JavaScript
	ButtonStyle      string // Bootstrap button style (primary, secondary, etc.)
	ButtonConfirmMsg string // Confirmation dialog text
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// SortElementsHierarchically sorts elements so that children appear immediately
// after their parent, respecting z_order within each level. This creates a flat
// list suitable for display in a table while preserving hierarchy.
func SortElementsHierarchically(elements []FormElement) []FormElement {
	if len(elements) == 0 {
		return elements
	}

	// Build a map of parent_id -> children, sorted by z_order
	childrenOf := make(map[int64][]FormElement)
	var roots []FormElement

	for _, el := range elements {
		if el.ParentID == nil {
			roots = append(roots, el)
		} else {
			childrenOf[*el.ParentID] = append(childrenOf[*el.ParentID], el)
		}
	}

	// Sort roots by z_order
	sortByZOrder(roots)

	// Sort each children group by z_order
	for parentID := range childrenOf {
		sortByZOrder(childrenOf[parentID])
	}

	// Recursively flatten: for each root, add it, then add all descendants
	var result []FormElement
	var addWithChildren func(el FormElement)
	addWithChildren = func(el FormElement) {
		result = append(result, el)
		for _, child := range childrenOf[el.ID] {
			addWithChildren(child)
		}
	}

	for _, root := range roots {
		addWithChildren(root)
	}

	return result
}

// sortByZOrder sorts a slice of FormElement by ZOrder in place.
func sortByZOrder(elements []FormElement) {
	for i := 0; i < len(elements)-1; i++ {
		for j := i + 1; j < len(elements); j++ {
			if elements[j].ZOrder < elements[i].ZOrder {
				elements[i], elements[j] = elements[j], elements[i]
			}
		}
	}
}

// CreateForm creates a new form.
func (s *SQLite) CreateForm(machineName, label, description string, eavEntityTypeID *int64) (*Form, error) {
	refID := utils.NewOpaqueID()
	const sqlInsert = `
		INSERT INTO forms (reference_id, machine_name, label, description, eav_entity_type_id, hide_submit_button, hide_cancel_button, hide_title, show_system_info)
		VALUES (?, ?, ?, ?, ?, 0, 0, 0, 0)
		RETURNING id, reference_id, machine_name, label, description, eav_entity_type_id, hide_submit_button, hide_cancel_button, hide_title, show_system_info, created_at, updated_at
	`

	var f Form
	err := s.QueryRowRW(sqlInsert, refID, machineName, label, description, eavEntityTypeID).Scan(
		&f.ID, &f.ReferenceID, &f.MachineName, &f.Label, &f.Description,
		&f.EAVEntityTypeID, &f.HideSubmitButton, &f.HideCancelButton, &f.HideTitle, &f.ShowSystemInfo, &f.CreatedAt, &f.UpdatedAt,
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
		       hide_submit_button, hide_cancel_button, hide_title, show_system_info, created_at, updated_at, deleted_at
		FROM forms
		WHERE reference_id = ? AND deleted_at IS NULL
	`

	var f Form
	err := s.QueryRow(q, refID).Scan(
		&f.ID, &f.ReferenceID, &f.MachineName, &f.Label, &f.Description,
		&f.EAVEntityTypeID, &f.HideSubmitButton, &f.HideCancelButton, &f.HideTitle, &f.ShowSystemInfo, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
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
func (s *SQLite) GetFormByMachineName(machineName string) (*Form, error) {
	const q = `
		SELECT id, reference_id, machine_name, label, description, eav_entity_type_id,
		       hide_submit_button, hide_cancel_button, hide_title, show_system_info, created_at, updated_at, deleted_at
		FROM forms
		WHERE machine_name = ? AND deleted_at IS NULL
	`

	var f Form
	err := s.QueryRow(q, machineName).Scan(
		&f.ID, &f.ReferenceID, &f.MachineName, &f.Label, &f.Description,
		&f.EAVEntityTypeID, &f.HideSubmitButton, &f.HideCancelButton, &f.HideTitle, &f.ShowSystemInfo, &f.CreatedAt, &f.UpdatedAt, &f.DeletedAt,
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
func (s *SQLite) ListForms() ([]Form, error) {
	const q = `
		SELECT id, reference_id, machine_name, label, description, eav_entity_type_id,
		       hide_submit_button, hide_cancel_button, hide_title, show_system_info, created_at, updated_at
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
			&f.EAVEntityTypeID, &f.HideSubmitButton, &f.HideCancelButton, &f.HideTitle, &f.ShowSystemInfo, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan form: %w", err)
		}
		forms = append(forms, f)
	}
	return forms, rows.Err()
}

// UpdateForm updates a form's basic info.
func (s *SQLite) UpdateForm(id int64, machineName, label, description string, eavEntityTypeID *int64, hideSubmitButton, hideCancelButton, hideTitle, showSystemInfo bool) error {
	const q = `
		UPDATE forms
		SET machine_name = ?, label = ?, description = ?, eav_entity_type_id = ?, 
		    hide_submit_button = ?, hide_cancel_button = ?, hide_title = ?, show_system_info = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL
	`
	return s.Exec(q, machineName, label, description, eavEntityTypeID, hideSubmitButton, hideCancelButton, hideTitle, showSystemInfo, id)
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
) (*FormElement, error) {
	refID := utils.NewOpaqueID()
	// Default col_span to 12 if not set
	if colSpan < 1 || colSpan > 12 {
		colSpan = 12
	}
	const sqlInsert = `
		INSERT INTO form_elements (
			reference_id, form_id, parent_id, machine_name, element_kind,
			label, help_text, z_order, col_span, alignment, ui_kind, ui_meta_json,
			eav_attribute_id, is_ui_only, is_readonly,
			button_filo_code, button_run_save, button_js_code, button_style, button_confirm_msg
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'left', ?, ?, ?, ?, ?, '', 0, '', 'primary', '')
		RETURNING id, reference_id, form_id, parent_id, machine_name, element_kind,
		          label, help_text, z_order, col_span, alignment, ui_kind, ui_meta_json,
		          eav_attribute_id, is_ui_only, is_readonly,
		          button_filo_code, button_run_save, button_js_code, button_style, button_confirm_msg,
		          created_at, updated_at
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
		refID, formID, parentIDVal, machineName, elementKind, label, helpText, zOrder, colSpan,
		uiKind, uiMetaJSON, eavAttrIDVal, boolToInt(isUIOnly), boolToInt(isReadonly),
	).Scan(
		&e.ID, &e.ReferenceID, &e.FormID, &scanParentID, &e.MachineName, &e.ElementKind,
		&e.Label, &e.HelpText, &e.ZOrder, &e.ColSpan, &e.Alignment, &e.UIKind, &e.UIMetaJSON,
		&scanEAVAttrID, &e.IsUIOnly, &e.IsReadonly,
		&e.ButtonFiloCode, &e.ButtonRunSave, &e.ButtonJSCode, &e.ButtonStyle, &e.ButtonConfirmMsg,
		&e.CreatedAt, &e.UpdatedAt,
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
		       label, help_text, z_order, col_span, alignment, ui_kind, ui_meta_json,
		       eav_attribute_id, is_ui_only, is_readonly,
		       button_filo_code, button_run_save, button_js_code, button_style, button_confirm_msg,
		       created_at, updated_at
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
			&e.Label, &e.HelpText, &e.ZOrder, &e.ColSpan, &e.Alignment, &e.UIKind, &e.UIMetaJSON,
			&eavAttrID, &e.IsUIOnly, &e.IsReadonly,
			&e.ButtonFiloCode, &e.ButtonRunSave, &e.ButtonJSCode, &e.ButtonStyle, &e.ButtonConfirmMsg,
			&e.CreatedAt, &e.UpdatedAt,
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
		       label, help_text, z_order, col_span, alignment, ui_kind, ui_meta_json,
		       eav_attribute_id, is_ui_only, is_readonly,
		       button_filo_code, button_run_save, button_js_code, button_style, button_confirm_msg,
		       created_at, updated_at
		FROM form_elements
		WHERE reference_id = ? AND deleted_at IS NULL
	`

	var e FormElement
	var parentID sql.NullInt64
	var eavAttrID sql.NullInt64

	err := s.QueryRow(q, refID).Scan(
		&e.ID, &e.ReferenceID, &e.FormID, &parentID, &e.MachineName, &e.ElementKind,
		&e.Label, &e.HelpText, &e.ZOrder, &e.ColSpan, &e.Alignment, &e.UIKind, &e.UIMetaJSON,
		&eavAttrID, &e.IsUIOnly, &e.IsReadonly,
		&e.ButtonFiloCode, &e.ButtonRunSave, &e.ButtonJSCode, &e.ButtonStyle, &e.ButtonConfirmMsg,
		&e.CreatedAt, &e.UpdatedAt,
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
	isUIOnly, isReadonly bool,
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
	const q = `
		UPDATE form_elements SET
			parent_id = ?,
			machine_name = ?,
			element_kind = ?,
			label = ?,
			help_text = ?,
			z_order = ?,
			col_span = ?,
			alignment = ?,
			ui_kind = ?,
			ui_meta_json = ?,
			eav_attribute_id = ?,
			is_ui_only = ?,
			is_readonly = ?,
			button_filo_code = ?,
			button_run_save = ?,
			button_js_code = ?,
			button_style = ?,
			button_confirm_msg = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL
	`

	var parentIDVal sql.NullInt64
	if parentID != nil {
		parentIDVal = sql.NullInt64{Int64: *parentID, Valid: true}
	}
	var eavAttrIDVal sql.NullInt64
	if eavAttributeID != nil {
		eavAttrIDVal = sql.NullInt64{Int64: *eavAttributeID, Valid: true}
	}

	return s.Exec(q,
		parentIDVal, machineName, elementKind, label, helpText,
		zOrder, colSpan, alignment, uiKind, uiMetaJSON, eavAttrIDVal,
		boolToInt(isUIOnly), boolToInt(isReadonly),
		buttonFiloCode, boolToInt(buttonRunSave), buttonJSCode, buttonStyle, buttonConfirmMsg,
		id,
	)
}

// ListGroupElements returns elements that can be parents (groups, accordions, cards, tabs).
func (s *SQLite) ListGroupElements(formID int64) ([]FormElement, error) {
	const q = `
		SELECT id, reference_id, form_id, parent_id, machine_name, element_kind,
		       label, help_text, z_order, col_span, alignment, ui_kind, ui_meta_json,
		       eav_attribute_id, is_ui_only, is_readonly, created_at, updated_at
		FROM form_elements
		WHERE form_id = ? AND deleted_at IS NULL
		  AND element_kind IN ('group', 'accordion', 'card', 'tabs')
		ORDER BY z_order, id
	`

	rows, err := s.Query(q, formID)
	if err != nil {
		return nil, fmt.Errorf("list group elements: %w", err)
	}
	defer rows.Close()

	var elements []FormElement
	for rows.Next() {
		var e FormElement
		var parentID sql.NullInt64
		var eavAttrID sql.NullInt64

		if err := rows.Scan(
			&e.ID, &e.ReferenceID, &e.FormID, &parentID, &e.MachineName, &e.ElementKind,
			&e.Label, &e.HelpText, &e.ZOrder, &e.ColSpan, &e.Alignment, &e.UIKind, &e.UIMetaJSON,
			&eavAttrID, &e.IsUIOnly, &e.IsReadonly, &e.CreatedAt, &e.UpdatedAt,
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
	const qCurrent = `SELECT form_id, z_order, parent_id FROM form_elements WHERE id = ? AND deleted_at IS NULL`
	if err := tx.QueryRow(qCurrent, elementID).Scan(&formID, &currentZOrder, &parentID); err != nil {
		return fmt.Errorf("get current element: %w", err)
	}

	// Find previous element (same parent, lower z_order, max z_order less than current)
	var prevID int64
	var prevZOrder int
	var qPrev string
	var args []interface{}
	if parentID.Valid {
		qPrev = `SELECT id, z_order FROM form_elements 
			WHERE form_id = ? AND parent_id = ? AND z_order < ? AND deleted_at IS NULL
			ORDER BY z_order DESC LIMIT 1`
		args = []interface{}{formID, parentID.Int64, currentZOrder}
	} else {
		qPrev = `SELECT id, z_order FROM form_elements 
			WHERE form_id = ? AND parent_id IS NULL AND z_order < ? AND deleted_at IS NULL
			ORDER BY z_order DESC LIMIT 1`
		args = []interface{}{formID, currentZOrder}
	}
	if err := tx.QueryRow(qPrev, args...).Scan(&prevID, &prevZOrder); err != nil {
		if err == sql.ErrNoRows {
			return nil // Already at top
		}
		return fmt.Errorf("find previous element: %w", err)
	}

	// Swap z_order values
	const qUpdate = `UPDATE form_elements SET z_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
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
	const qCurrent = `SELECT form_id, z_order, parent_id FROM form_elements WHERE id = ? AND deleted_at IS NULL`
	if err := tx.QueryRow(qCurrent, elementID).Scan(&formID, &currentZOrder, &parentID); err != nil {
		return fmt.Errorf("get current element: %w", err)
	}

	// Find next element (same parent, higher z_order, min z_order greater than current)
	var nextID int64
	var nextZOrder int
	var qNext string
	var args []interface{}
	if parentID.Valid {
		qNext = `SELECT id, z_order FROM form_elements 
			WHERE form_id = ? AND parent_id = ? AND z_order > ? AND deleted_at IS NULL
			ORDER BY z_order ASC LIMIT 1`
		args = []interface{}{formID, parentID.Int64, currentZOrder}
	} else {
		qNext = `SELECT id, z_order FROM form_elements 
			WHERE form_id = ? AND parent_id IS NULL AND z_order > ? AND deleted_at IS NULL
			ORDER BY z_order ASC LIMIT 1`
		args = []interface{}{formID, currentZOrder}
	}
	if err := tx.QueryRow(qNext, args...).Scan(&nextID, &nextZOrder); err != nil {
		if err == sql.ErrNoRows {
			return nil // Already at bottom
		}
		return fmt.Errorf("find next element: %w", err)
	}

	// Swap z_order values
	const qUpdate = `UPDATE form_elements SET z_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	if err := tx.Exec(qUpdate, nextZOrder, elementID); err != nil {
		return fmt.Errorf("update current z_order: %w", err)
	}
	if err := tx.Exec(qUpdate, currentZOrder, nextID); err != nil {
		return fmt.Errorf("update next z_order: %w", err)
	}

	return tx.Commit()
}
