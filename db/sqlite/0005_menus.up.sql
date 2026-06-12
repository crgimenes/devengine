-- 0005_menus.up.sql
-- Menus / Navigation layer (single-tenant).
-- Menus are reusable navigation components that can be assigned to forms.
-- Menu items are recursive: any item can contain other items via parent_id.

PRAGMA foreign_keys = ON;

-- ----------------------------------------------------------------------
-- menus
-- A menu is a reusable navigation component.
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS menus (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT    NOT NULL,

    machine_name  TEXT    NOT NULL COLLATE NOCASE, -- stable key for routing
    label         TEXT    NOT NULL,
    description   TEXT,

    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME
);

-- Partial unique indexes (exclude soft-deleted rows)
CREATE UNIQUE INDEX IF NOT EXISTS idx_menus_reference_id_active
    ON menus(reference_id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_menus_machine_name_active
    ON menus(machine_name COLLATE NOCASE) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_menus_deleted_at
    ON menus(deleted_at);

-- Trigger for reference_id generation
CREATE TRIGGER IF NOT EXISTS trg_menus_reference_id
AFTER INSERT ON menus
WHEN NEW.reference_id IS NULL OR NEW.reference_id = ''
BEGIN
    UPDATE menus 
    SET reference_id = lower(hex(randomblob(24)))
    WHERE id = NEW.id;
END;

-- ----------------------------------------------------------------------
-- menu_items
-- Menu items are recursive via parent_id (submenus).
-- item_type: 'link' (default), 'separator', 'submenu'
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS menu_items (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT    NOT NULL,

    menu_id       INTEGER NOT NULL
        REFERENCES menus(id) ON DELETE CASCADE,

    -- Recursive: parent item (NULL = root level)
    parent_id     INTEGER
        REFERENCES menu_items(id) ON DELETE CASCADE,

    -- Stable identifier within the menu
    machine_name  TEXT    NOT NULL COLLATE NOCASE,

    -- Display
    label         TEXT    NOT NULL,
    icon          TEXT,  -- Optional Bootstrap icon class (e.g., 'bi-house')

    -- Item type: 'link', 'separator', 'submenu'
    item_type     TEXT    NOT NULL DEFAULT 'link' CHECK (item_type IN ('link', 'separator', 'submenu')),

    -- URL for links (optional, used when item_type = 'link')
    url           TEXT,

    -- JavaScript code to execute client-side (requires external JS file for CSP)
    js_code       TEXT,

    -- Filo code to execute server-side
    filo_code     TEXT,

    -- Ordering within parent
    z_order       INTEGER NOT NULL DEFAULT 0,

    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME,

    -- Unique machine_name per menu
    UNIQUE(menu_id, machine_name)
);

-- Partial unique indexes (exclude soft-deleted rows)
CREATE UNIQUE INDEX IF NOT EXISTS idx_menu_items_reference_id_active
    ON menu_items(reference_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_menu_items_menu_order
    ON menu_items(menu_id, parent_id, z_order, id);

CREATE INDEX IF NOT EXISTS idx_menu_items_parent_order
    ON menu_items(parent_id, z_order, id);

CREATE INDEX IF NOT EXISTS idx_menu_items_deleted_at
    ON menu_items(deleted_at);

-- Trigger for reference_id generation
CREATE TRIGGER IF NOT EXISTS trg_menu_items_reference_id
AFTER INSERT ON menu_items
WHEN NEW.reference_id IS NULL OR NEW.reference_id = ''
BEGIN
    UPDATE menu_items 
    SET reference_id = lower(hex(randomblob(24)))
    WHERE id = NEW.id;
END;

-- ----------------------------------------------------------------------
-- Notes:
-- - Menus can be assigned to forms to customize the navbar in runtime.
-- - item_type 'submenu' items should have children via parent_id.
-- - item_type 'separator' items are visual dividers.
-- - icon uses Bootstrap Icons class names (e.g., 'bi-house', 'bi-gear').
-- - Actions (Filo scripts, etc.) will be added in a future phase.
-- ----------------------------------------------------------------------
