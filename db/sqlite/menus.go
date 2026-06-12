package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/utils"
)

// CreateMenu creates a new menu.
func (s *SQLite) CreateMenu(machineName, label, description string) (*db.Menu, error) {
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

	var m db.Menu
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
func (s *SQLite) GetMenuByRefID(refID string) (*db.Menu, error) {
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

	var m db.Menu
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
func (s *SQLite) GetMenuByID(id int64) (*db.Menu, error) {
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

	var m db.Menu
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
func (s *SQLite) GetMenuByMachineName(machineName string) (*db.Menu, error) {
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

	var m db.Menu
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
func (s *SQLite) ListMenus() ([]db.Menu, error) {
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

	var menus []db.Menu
	for rows.Next() {
		var m db.Menu
		err := rows.Scan(
			&m.ID,          // 1
			&m.ReferenceID, // 2
			&m.MachineName, // 3
			&m.Label,       // 4
			&m.Description, // 5
			&m.CreatedAt,   // 6
			&m.UpdatedAt,   // 7
		)
		if err != nil {
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
) (*db.MenuItem, error) {
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

	var item db.MenuItem
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
func (s *SQLite) ListMenuItems(menuID int64) ([]db.MenuItem, error) {
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

	var items []db.MenuItem
	for rows.Next() {
		var item db.MenuItem
		var parentID sql.NullInt64
		var icon sql.NullString
		var url sql.NullString
		var jsCode sql.NullString
		var filoCode sql.NullString

		err := rows.Scan(
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
func (s *SQLite) GetMenuItemByRefID(refID string) (*db.MenuItem, error) {
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

	var item db.MenuItem
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
func (s *SQLite) ListSubmenuItems(menuID int64) ([]db.MenuItem, error) {
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

	var items []db.MenuItem
	for rows.Next() {
		var item db.MenuItem
		var parentID sql.NullInt64
		var icon sql.NullString
		var url sql.NullString
		var jsCode sql.NullString
		var filoCode sql.NullString

		err := rows.Scan(
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
	err = tx.QueryRow(qCurrent, itemID).Scan(
		&menuID,        // 1
		&currentZOrder, // 2
		&parentID,      // 3
	)
	if err != nil {
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
	err = tx.QueryRow(qPrev, args...).Scan(
		&prevID,     // 1
		&prevZOrder, // 2
	)
	if err != nil {
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
	err = tx.Exec(qUpdate, prevZOrder, itemID)
	if err != nil {
		return fmt.Errorf("update current z_order: %w", err)
	}
	err = tx.Exec(qUpdate, currentZOrder, prevID)
	if err != nil {
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
	err = tx.QueryRow(qCurrent, itemID).Scan(
		&menuID,        // 1
		&currentZOrder, // 2
		&parentID,      // 3
	)
	if err != nil {
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
	err = tx.QueryRow(qNext, args...).Scan(
		&nextID,     // 1
		&nextZOrder, // 2
	)
	if err != nil {
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
	err = tx.Exec(qUpdate, nextZOrder, itemID)
	if err != nil {
		return fmt.Errorf("update current z_order: %w", err)
	}
	err = tx.Exec(qUpdate, currentZOrder, nextID)
	if err != nil {
		return fmt.Errorf("update next z_order: %w", err)
	}

	return tx.Commit()
}
