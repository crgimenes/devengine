package db

import (
	"time"
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
	MenuID           *int64 // Associated menu for navbar display when form is active
	IsSearch         bool   // When true, /form/{name} opens the record listing (search view)
	ExposeAPI        bool   // When true, the form is reachable as a REST API under /api/v1
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
	HideLabel      bool
	HideHelpText   bool
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
