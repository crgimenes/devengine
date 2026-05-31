package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/crgimenes/devengine/utils"
)

// Menu represents a menu definition.
type Menu struct {
	ID          int64
	ReferenceID string
	MachineName string
	Label       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// MenuItem represents a menu item.
type MenuItem struct {
	ID          int64
	ReferenceID string
	MenuID      int64
	ParentID    *int64 // NULL for root-level items
	MachineName string
	Label       string
	Icon        string // Optional Bootstrap icon class
	ItemType    string // 'link', 'separator', 'submenu'
	URL         string // Optional, for links
	JSCode      string // JavaScript code to execute client-side
	FiloCode    string // Filo code to execute server-side
	ZOrder      int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// SortMenuItemsHierarchically sorts items so that children appear immediately
// after their parent, respecting z_order within each level.
func SortMenuItemsHierarchically(items []MenuItem) []MenuItem {
	if len(items) == 0 {
		return items
	}

	// Build a map of parent_id -> children, sorted by z_order
	childrenOf := make(map[int64][]MenuItem)
	var roots []MenuItem

	for _, item := range items {
		if item.ParentID == nil {
			roots = append(roots, item)
		} else {
			childrenOf[*item.ParentID] = append(childrenOf[*item.ParentID], item)
		}
	}

	// Sort roots by z_order
	sortMenuItemsByZOrder(roots)

	// Sort each children group by z_order
	for parentID := range childrenOf {
		sortMenuItemsByZOrder(childrenOf[parentID])
	}

	// Recursively flatten: for each root, add it, then add all descendants
	var result []MenuItem
	var addWithChildren func(item MenuItem)
	addWithChildren = func(item MenuItem) {
		result = append(result, item)
		for _, child := range childrenOf[item.ID] {
			addWithChildren(child)
		}
	}

	for _, root := range roots {
		addWithChildren(root)
	}

	return result
}

// sortMenuItemsByZOrder sorts a slice of MenuItem by ZOrder in place.
func sortMenuItemsByZOrder(items []MenuItem) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].ZOrder < items[i].ZOrder ||
				(items[j].ZOrder == items[i].ZOrder && items[j].MachineName < items[i].MachineName) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// DescendantItem represents a menu item with its depth in the hierarchy.
type DescendantItem struct {
	MenuItem
	Depth int
}

// CollectMenuDescendants recursively collects all descendants of a given parent ID.
// It includes cycle detection: if an item ID is visited more than once, its children
// are not processed to prevent infinite recursion.
func CollectMenuDescendants(items []MenuItem, parentID int64) []DescendantItem {
	// Build a map of parent_id -> children
	childrenOf := make(map[int64][]MenuItem)
	for _, item := range items {
		if item.ParentID != nil {
			childrenOf[*item.ParentID] = append(childrenOf[*item.ParentID], item)
		}
	}

	// Track visited IDs to prevent cycles
	visited := make(map[int64]bool)

	var collect func(parentID int64, depth int) []DescendantItem
	collect = func(parentID int64, depth int) []DescendantItem {
		var result []DescendantItem
		for _, child := range childrenOf[parentID] {
			// Cycle detection: skip if already visited
			if visited[child.ID] {
				continue
			}
			visited[child.ID] = true

			result = append(result, DescendantItem{MenuItem: child, Depth: depth})
			// Recursively add grandchildren, etc.
			result = append(result, collect(child.ID, depth+1)...)
		}
		return result
	}

	return collect(parentID, 1)
}

// GetDirectChildren returns only the direct children of a parent ID from the items slice.
func GetDirectChildren(items []MenuItem, parentID int64) []MenuItem {
	var children []MenuItem
	for _, item := range items {
		if item.ParentID != nil && *item.ParentID == parentID {
			children = append(children, item)
		}
	}
	return children
}

// MenuItemNode represents a menu item with its children as a tree structure.
type MenuItemNode struct {
	MenuItem
	Children        []MenuItemNode
	MenuMachineName string // Menu machine name for action URLs
}

// BuildMenuItemTree builds a tree structure for menu items.
// It returns only the root-level items (items with no parent), each with their children populated recursively.
func BuildMenuItemTree(items []MenuItem) []MenuItemNode {
	return BuildMenuItemTreeWithName(items, "")
}

// BuildMenuItemTreeWithName builds a tree structure with the menu machine name propagated to all nodes.
func BuildMenuItemTreeWithName(items []MenuItem, menuMachineName string) []MenuItemNode {
	// Build a map of parent_id -> children
	childrenOf := make(map[int64][]MenuItem)
	for _, item := range items {
		if item.ParentID != nil {
			childrenOf[*item.ParentID] = append(childrenOf[*item.ParentID], item)
		}
	}

	// Track visited IDs to prevent cycles
	visited := make(map[int64]bool)

	var buildNode func(item MenuItem) MenuItemNode
	buildNode = func(item MenuItem) MenuItemNode {
		node := MenuItemNode{MenuItem: item, MenuMachineName: menuMachineName}

		// Cycle detection
		if visited[item.ID] {
			return node
		}
		visited[item.ID] = true

		// Add children recursively
		for _, child := range childrenOf[item.ID] {
			node.Children = append(node.Children, buildNode(child))
		}
		return node
	}

	// Build tree from root items only
	var roots []MenuItemNode
	for _, item := range items {
		if item.ParentID == nil {
			roots = append(roots, buildNode(item))
		}
	}
	return roots
}

// CreateMenu creates a new menu.
func (s *SQLite) CreateMenu(machineName, label, description string) (*Menu, error) {
	refID := utils.NewOpaqueID()
	const sqlInsert = `INSERT INTO menus (
		reference_id, -- 1
		machine_name, -- 2
		label,        -- 3
		description   -- 4
	) VALUES (
		?, -- 1
		?, -- 2
		?, -- 3
		?  -- 4
	)
	RETURNING
		id,           -- 1
		reference_id, -- 2
		machine_name, -- 3
		label,        -- 4
		description,  -- 5
		created_at,   -- 6
		updated_at    -- 7
	`

	var m Menu
	err := s.QueryRowRW(
		sqlInsert,
		refID,       // 1
		machineName, // 2
		label,       // 3
		description, // 4
	).Scan(
		&m.ID,          // 1
		&m.ReferenceID, // 2
		&m.MachineName, // 3
		&m.Label,       // 4
		&m.Description, // 5
		&m.CreatedAt,   // 6
		&m.UpdatedAt,   // 7
	)
	if err != nil {
		return nil, fmt.Errorf("create menu: %w", err)
	}
	return &m, nil
}

// GetMenuByRefID retrieves a menu by reference_id.
func (s *SQLite) GetMenuByRefID(refID string) (*Menu, error) {
	const q = `SELECT
		id,           -- 1
		reference_id, -- 2
		machine_name, -- 3
		label,        -- 4
		description,  -- 5
		created_at,   -- 6
		updated_at,   -- 7
		deleted_at    -- 8
	FROM menus
	WHERE reference_id = ? -- 1
	AND deleted_at IS NULL`

	var m Menu
	err := s.QueryRow(
		q,
		refID, // 1
	).Scan(
		&m.ID,          // 1
		&m.ReferenceID, // 2
		&m.MachineName, // 3
		&m.Label,       // 4
		&m.Description, // 5
		&m.CreatedAt,   // 6
		&m.UpdatedAt,   // 7
		&m.DeletedAt,   // 8
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("menu not found: %s", refID)
		}
		return nil, fmt.Errorf("get menu by ref id: %w", err)
	}
	return &m, nil
}

// GetMenuByID retrieves a menu by its ID.
// Returns nil, nil if not found.
func (s *SQLite) GetMenuByID(id int64) (*Menu, error) {
	const q = `SELECT
		id,           -- 1
		reference_id, -- 2
		machine_name, -- 3
		label,        -- 4
		description,  -- 5
		created_at,   -- 6
		updated_at,   -- 7
		deleted_at    -- 8
	FROM menus
	WHERE id = ?           -- 1
	AND deleted_at IS NULL`

	var m Menu
	err := s.QueryRow(
		q,
		id, // 1
	).Scan(
		&m.ID,          // 1
		&m.ReferenceID, // 2
		&m.MachineName, // 3
		&m.Label,       // 4
		&m.Description, // 5
		&m.CreatedAt,   // 6
		&m.UpdatedAt,   // 7
		&m.DeletedAt,   // 8
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("get menu by id: %w", err)
	}
	return &m, nil
}

// GetMenuByMachineName retrieves a menu by machine_name.
// Returns nil, nil if not found (soft not-found to allow fallback logic).
func (s *SQLite) GetMenuByMachineName(machineName string) (*Menu, error) {
	const q = `SELECT
		id,           -- 1
		reference_id, -- 2
		machine_name, -- 3
		label,        -- 4
		description,  -- 5
		created_at,   -- 6
		updated_at,   -- 7
		deleted_at    -- 8
	FROM menus
	WHERE machine_name = ? -- 1
	AND deleted_at IS NULL`

	var m Menu
	err := s.QueryRow(
		q,
		machineName, // 1
	).Scan(
		&m.ID,          // 1
		&m.ReferenceID, // 2
		&m.MachineName, // 3
		&m.Label,       // 4
		&m.Description, // 5
		&m.CreatedAt,   // 6
		&m.UpdatedAt,   // 7
		&m.DeletedAt,   // 8
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found is OK, caller will use fallback
		}
		return nil, fmt.Errorf("get menu by machine name: %w", err)
	}
	return &m, nil
}

// ListMenus returns all non-deleted menus.
func (s *SQLite) ListMenus() ([]Menu, error) {
	const q = `SELECT
		id,           -- 1
		reference_id, -- 2
		machine_name, -- 3
		label,        -- 4
		description,  -- 5
		created_at,   -- 6
		updated_at    -- 7
	FROM menus
	WHERE deleted_at IS NULL
	ORDER BY label`

	rows, err := s.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list menus: %w", err)
	}
	defer rows.Close()

	var menus []Menu
	for rows.Next() {
		var m Menu
		if err := rows.Scan(
			&m.ID,          // 1
			&m.ReferenceID, // 2
			&m.MachineName, // 3
			&m.Label,       // 4
			&m.Description, // 5
			&m.CreatedAt,   // 6
			&m.UpdatedAt,   // 7
		); err != nil {
			return nil, fmt.Errorf("scan menu: %w", err)
		}
		menus = append(menus, m)
	}
	return menus, rows.Err()
}

// UpdateMenu updates a menu's basic info.
func (s *SQLite) UpdateMenu(id int64, machineName, label, description string) error {
	const q = `UPDATE menus
	SET
		machine_name = ?, -- 1
		label = ?,        -- 2
		description = ?,  -- 3
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?          -- 4
	AND deleted_at IS NULL`
	return s.Exec(
		q,
		machineName, // 1
		label,       // 2
		description, // 3
		id,          // 4
	)
}

// SoftDeleteMenu soft-deletes a menu.
func (s *SQLite) SoftDeleteMenu(id int64) error {
	const q = `UPDATE menus SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`
	return s.Exec(q, id)
}

// CreateMenuItem creates a new menu item.
func (s *SQLite) CreateMenuItem(
	menuID int64,
	parentID *int64,
	machineName, label, icon, itemType, url, jsCode, filoCode string,
	zOrder int,
) (*MenuItem, error) {
	refID := utils.NewOpaqueID()
	if itemType == "" {
		itemType = "link"
	}
	const sqlInsert = `INSERT INTO menu_items (
		reference_id, -- 1
		menu_id,      -- 2
		parent_id,    -- 3
		machine_name, -- 4
		label,        -- 5
		icon,         -- 6
		item_type,    -- 7
		url,          -- 8
		js_code,      -- 9
		filo_code,    -- 10
		z_order       -- 11
	) VALUES (
		?,  -- 1
		?,  -- 2
		?,  -- 3
		?,  -- 4
		?,  -- 5
		?,  -- 6
		?,  -- 7
		?,  -- 8
		?,  -- 9
		?,  -- 10
		?   -- 11
	)
	RETURNING
		id,           -- 1
		reference_id, -- 2
		menu_id,      -- 3
		parent_id,    -- 4
		machine_name, -- 5
		label,        -- 6
		icon,         -- 7
		item_type,    -- 8
		url,          -- 9
		js_code,      -- 10
		filo_code,    -- 11
		z_order,      -- 12
		created_at,   -- 13
		updated_at    -- 14
	`

	var item MenuItem
	var parentIDVal sql.NullInt64
	if parentID != nil {
		parentIDVal = sql.NullInt64{Int64: *parentID, Valid: true}
	}

	var scanParentID sql.NullInt64
	var scanIcon sql.NullString
	var scanURL sql.NullString
	var scanJSCode sql.NullString
	var scanFiloCode sql.NullString

	err := s.QueryRowRW(
		sqlInsert,
		refID,       // 1
		menuID,      // 2
		parentIDVal, // 3
		machineName, // 4
		label,       // 5
		icon,        // 6
		itemType,    // 7
		url,         // 8
		jsCode,      // 9
		filoCode,    // 10
		zOrder,      // 11
	).Scan(
		&item.ID,          // 1
		&item.ReferenceID, // 2
		&item.MenuID,      // 3
		&scanParentID,     // 4
		&item.MachineName, // 5
		&item.Label,       // 6
		&scanIcon,         // 7
		&item.ItemType,    // 8
		&scanURL,          // 9
		&scanJSCode,       // 10
		&scanFiloCode,     // 11
		&item.ZOrder,      // 12
		&item.CreatedAt,   // 13
		&item.UpdatedAt,   // 14
	)
	if err != nil {
		return nil, fmt.Errorf("create menu item: %w", err)
	}

	if scanParentID.Valid {
		item.ParentID = &scanParentID.Int64
	}
	if scanIcon.Valid {
		item.Icon = scanIcon.String
	}
	if scanURL.Valid {
		item.URL = scanURL.String
	}
	if scanJSCode.Valid {
		item.JSCode = scanJSCode.String
	}
	if scanFiloCode.Valid {
		item.FiloCode = scanFiloCode.String
	}

	return &item, nil
}

// ListMenuItems returns all non-deleted items for a menu, ordered by z_order.
func (s *SQLite) ListMenuItems(menuID int64) ([]MenuItem, error) {
	const q = `SELECT
		id,           -- 1
		reference_id, -- 2
		menu_id,      -- 3
		parent_id,    -- 4
		machine_name, -- 5
		label,        -- 6
		icon,         -- 7
		item_type,    -- 8
		url,          -- 9
		js_code,      -- 10
		filo_code,    -- 11
		z_order,      -- 12
		created_at,   -- 13
		updated_at    -- 14
	FROM menu_items
	WHERE menu_id = ?     -- 1
	AND deleted_at IS NULL
	ORDER BY COALESCE(parent_id, 0), z_order, machine_name, id`

	rows, err := s.Query(
		q,
		menuID, // 1
	)
	if err != nil {
		return nil, fmt.Errorf("list menu items: %w", err)
	}
	defer rows.Close()

	var items []MenuItem
	for rows.Next() {
		var item MenuItem
		var parentID sql.NullInt64
		var icon sql.NullString
		var url sql.NullString
		var jsCode sql.NullString
		var filoCode sql.NullString

		if err := rows.Scan(
			&item.ID,          // 1
			&item.ReferenceID, // 2
			&item.MenuID,      // 3
			&parentID,         // 4
			&item.MachineName, // 5
			&item.Label,       // 6
			&icon,             // 7
			&item.ItemType,    // 8
			&url,              // 9
			&jsCode,           // 10
			&filoCode,         // 11
			&item.ZOrder,      // 12
			&item.CreatedAt,   // 13
			&item.UpdatedAt,   // 14
		); err != nil {
			return nil, fmt.Errorf("scan menu item: %w", err)
		}

		if parentID.Valid {
			item.ParentID = &parentID.Int64
		}
		if icon.Valid {
			item.Icon = icon.String
		}
		if url.Valid {
			item.URL = url.String
		}
		if jsCode.Valid {
			item.JSCode = jsCode.String
		}
		if filoCode.Valid {
			item.FiloCode = filoCode.String
		}

		items = append(items, item)
	}
	return items, rows.Err()
}

// GetMenuItemByRefID retrieves a menu item by reference_id.
func (s *SQLite) GetMenuItemByRefID(refID string) (*MenuItem, error) {
	const q = `SELECT
		id,           -- 1
		reference_id, -- 2
		menu_id,      -- 3
		parent_id,    -- 4
		machine_name, -- 5
		label,        -- 6
		icon,         -- 7
		item_type,    -- 8
		url,          -- 9
		js_code,      -- 10
		filo_code,    -- 11
		z_order,      -- 12
		created_at,   -- 13
		updated_at    -- 14
	FROM menu_items
	WHERE reference_id = ? -- 1
	AND deleted_at IS NULL`

	var item MenuItem
	var parentID sql.NullInt64
	var icon sql.NullString
	var url sql.NullString
	var jsCode sql.NullString
	var filoCode sql.NullString

	err := s.QueryRow(
		q,
		refID, // 1
	).Scan(
		&item.ID,          // 1
		&item.ReferenceID, // 2
		&item.MenuID,      // 3
		&parentID,         // 4
		&item.MachineName, // 5
		&item.Label,       // 6
		&icon,             // 7
		&item.ItemType,    // 8
		&url,              // 9
		&jsCode,           // 10
		&filoCode,         // 11
		&item.ZOrder,      // 12
		&item.CreatedAt,   // 13
		&item.UpdatedAt,   // 14
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("menu item not found: %s", refID)
		}
		return nil, fmt.Errorf("get menu item by ref id: %w", err)
	}

	if parentID.Valid {
		item.ParentID = &parentID.Int64
	}
	if icon.Valid {
		item.Icon = icon.String
	}
	if url.Valid {
		item.URL = url.String
	}
	if jsCode.Valid {
		item.JSCode = jsCode.String
	}
	if filoCode.Valid {
		item.FiloCode = filoCode.String
	}

	return &item, nil
}

// UpdateMenuItem updates an existing menu item.
func (s *SQLite) UpdateMenuItem(
	id int64,
	parentID *int64,
	machineName, label, icon, itemType, url, jsCode, filoCode string,
	zOrder int,
) error {
	if itemType == "" {
		itemType = "link"
	}
	const q = `UPDATE menu_items
	SET
		parent_id = ?,    -- 1
		machine_name = ?, -- 2
		label = ?,        -- 3
		icon = ?,         -- 4
		item_type = ?,    -- 5
		url = ?,          -- 6
		js_code = ?,      -- 7
		filo_code = ?,    -- 8
		z_order = ?,      -- 9
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?          -- 10
	AND deleted_at IS NULL`

	var parentIDVal sql.NullInt64
	if parentID != nil {
		parentIDVal = sql.NullInt64{Int64: *parentID, Valid: true}
	}

	return s.Exec(
		q,
		parentIDVal, // 1
		machineName, // 2
		label,       // 3
		icon,        // 4
		itemType,    // 5
		url,         // 6
		jsCode,      // 7
		filoCode,    // 8
		zOrder,      // 9
		id,          // 10
	)
}

// DeleteMenuItem permanently deletes a menu item.
func (s *SQLite) DeleteMenuItem(id int64) error {
	const q = `DELETE FROM menu_items WHERE id = ?`
	return s.Exec(q, id)
}

// ListSubmenuItems returns items that can be parents (type 'submenu').
func (s *SQLite) ListSubmenuItems(menuID int64) ([]MenuItem, error) {
	const q = `SELECT
		id,           -- 1
		reference_id, -- 2
		menu_id,      -- 3
		parent_id,    -- 4
		machine_name, -- 5
		label,        -- 6
		icon,         -- 7
		item_type,    -- 8
		url,          -- 9
		js_code,      -- 10
		filo_code,    -- 11
		z_order,      -- 12
		created_at,   -- 13
		updated_at    -- 14
	FROM menu_items
	WHERE menu_id = ?     -- 1
	AND deleted_at IS NULL
	AND item_type = 'submenu'
	ORDER BY z_order, machine_name, id`

	rows, err := s.Query(
		q,
		menuID, // 1
	)
	if err != nil {
		return nil, fmt.Errorf("list submenu items: %w", err)
	}
	defer rows.Close()

	var items []MenuItem
	for rows.Next() {
		var item MenuItem
		var parentID sql.NullInt64
		var icon sql.NullString
		var url sql.NullString
		var jsCode sql.NullString
		var filoCode sql.NullString

		if err := rows.Scan(
			&item.ID,          // 1
			&item.ReferenceID, // 2
			&item.MenuID,      // 3
			&parentID,         // 4
			&item.MachineName, // 5
			&item.Label,       // 6
			&icon,             // 7
			&item.ItemType,    // 8
			&url,              // 9
			&jsCode,           // 10
			&filoCode,         // 11
			&item.ZOrder,      // 12
			&item.CreatedAt,   // 13
			&item.UpdatedAt,   // 14
		); err != nil {
			return nil, fmt.Errorf("scan submenu item: %w", err)
		}

		if parentID.Valid {
			item.ParentID = &parentID.Int64
		}
		if icon.Valid {
			item.Icon = icon.String
		}
		if url.Valid {
			item.URL = url.String
		}
		if jsCode.Valid {
			item.JSCode = jsCode.String
		}
		if filoCode.Valid {
			item.FiloCode = filoCode.String
		}

		items = append(items, item)
	}
	return items, rows.Err()
}

// MoveMenuItemUp swaps this item's z_order with the previous item (lower z_order, same parent).
func (s *SQLite) MoveMenuItemUp(itemID int64) error {
	tx, err := s.BeginTransaction()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get current item
	var menuID int64
	var currentZOrder int
	var parentID sql.NullInt64
	const qCurrent = `SELECT
		menu_id,   -- 1
		z_order,   -- 2
		parent_id  -- 3
	FROM menu_items
	WHERE id = ?   -- 1
	AND deleted_at IS NULL`
	if err := tx.QueryRow(qCurrent, itemID).Scan(
		&menuID,        // 1
		&currentZOrder, // 2
		&parentID,      // 3
	); err != nil {
		return fmt.Errorf("get current item: %w", err)
	}

	// Find previous item (same parent, lower z_order, max z_order less than current)
	var prevID int64
	var prevZOrder int
	var qPrev string
	var args []any
	if parentID.Valid {
		qPrev = `SELECT
			id,      -- 1
			z_order  -- 2
		FROM menu_items
		WHERE menu_id = ?   -- 1
		AND parent_id = ?   -- 2
		AND z_order < ?     -- 3
		AND deleted_at IS NULL
		ORDER BY z_order DESC LIMIT 1`
		args = []any{menuID, parentID.Int64, currentZOrder}
	} else {
		qPrev = `SELECT
			id,      -- 1
			z_order  -- 2
		FROM menu_items
		WHERE menu_id = ?   -- 1
		AND parent_id IS NULL
		AND z_order < ?     -- 2
		AND deleted_at IS NULL
		ORDER BY z_order DESC LIMIT 1`
		args = []any{menuID, currentZOrder}
	}
	if err := tx.QueryRow(qPrev, args...).Scan(
		&prevID,     // 1
		&prevZOrder, // 2
	); err != nil {
		if err == sql.ErrNoRows {
			return nil // Already at top
		}
		return fmt.Errorf("find previous item: %w", err)
	}

	// Swap z_order values
	const qUpdate = `UPDATE menu_items
	SET
		z_order = ?,  -- 1
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?      -- 2
	`
	if err := tx.Exec(qUpdate, prevZOrder, itemID); err != nil {
		return fmt.Errorf("update current z_order: %w", err)
	}
	if err := tx.Exec(qUpdate, currentZOrder, prevID); err != nil {
		return fmt.Errorf("update previous z_order: %w", err)
	}

	return tx.Commit()
}

// MoveMenuItemDown swaps this item's z_order with the next item (higher z_order, same parent).
func (s *SQLite) MoveMenuItemDown(itemID int64) error {
	tx, err := s.BeginTransaction()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get current item
	var menuID int64
	var currentZOrder int
	var parentID sql.NullInt64
	const qCurrent = `SELECT
		menu_id,   -- 1
		z_order,   -- 2
		parent_id  -- 3
	FROM menu_items
	WHERE id = ?   -- 1
	AND deleted_at IS NULL`
	if err := tx.QueryRow(qCurrent, itemID).Scan(
		&menuID,        // 1
		&currentZOrder, // 2
		&parentID,      // 3
	); err != nil {
		return fmt.Errorf("get current item: %w", err)
	}

	// Find next item (same parent, higher z_order, min z_order greater than current)
	var nextID int64
	var nextZOrder int
	var qNext string
	var args []any
	if parentID.Valid {
		qNext = `SELECT
			id,      -- 1
			z_order  -- 2
		FROM menu_items
		WHERE menu_id = ?   -- 1
		AND parent_id = ?   -- 2
		AND z_order > ?     -- 3
		AND deleted_at IS NULL
		ORDER BY z_order ASC LIMIT 1`
		args = []any{menuID, parentID.Int64, currentZOrder}
	} else {
		qNext = `SELECT
			id,      -- 1
			z_order  -- 2
		FROM menu_items
		WHERE menu_id = ?   -- 1
		AND parent_id IS NULL
		AND z_order > ?     -- 2
		AND deleted_at IS NULL
		ORDER BY z_order ASC LIMIT 1`
		args = []any{menuID, currentZOrder}
	}
	if err := tx.QueryRow(qNext, args...).Scan(
		&nextID,     // 1
		&nextZOrder, // 2
	); err != nil {
		if err == sql.ErrNoRows {
			return nil // Already at bottom
		}
		return fmt.Errorf("find next item: %w", err)
	}

	// Swap z_order values
	const qUpdate = `UPDATE menu_items
	SET
		z_order = ?,  -- 1
		updated_at = CURRENT_TIMESTAMP
	WHERE id = ?      -- 2
	`
	if err := tx.Exec(qUpdate, nextZOrder, itemID); err != nil {
		return fmt.Errorf("update current z_order: %w", err)
	}
	if err := tx.Exec(qUpdate, currentZOrder, nextID); err != nil {
		return fmt.Errorf("update next z_order: %w", err)
	}

	return tx.Commit()
}
