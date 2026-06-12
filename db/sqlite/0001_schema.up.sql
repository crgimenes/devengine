-- 0001_schema.up.sql
-- Complete devengine schema (SQLite dialect), unified pre-1.0.
-- While the engine is experimental there is no migration history: schema
-- changes edit this file in place and databases are recreated from scratch.
-- Real append-only migrations start at 1.0.0. Application migrations keep
-- using ids 1000-9999 on top of this file.

PRAGMA foreign_keys = ON;

-- Core user accounts, identities, magic links, and file storage (with FTS)

CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    reference_id TEXT NOT NULL DEFAULT '' UNIQUE, -- a trigger will set this to a UUID
    username TEXT UNIQUE COLLATE NOCASE,
    email TEXT UNIQUE COLLATE NOCASE,
    password_hash TEXT, -- nullable; populated by basic auth, may stay NULL when other methods are used
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0,1)),
    sysop INTEGER NOT NULL DEFAULT 0 CHECK (sysop IN (0,1)),
    avatar_url TEXT,
    -- Per-user UI language. Empty means "no preference": the request
    -- falls back to Accept-Language and then to the app default locale.
    locale TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_username_nocase
    ON users(LOWER(username)) WHERE username IS NOT NULL;
CREATE INDEX idx_users_email_nocase
    ON users(LOWER(email)) WHERE email IS NOT NULL;
CREATE INDEX idx_users_enabled ON users(enabled);
CREATE INDEX idx_users_reference_id ON users(reference_id);

CREATE TRIGGER users_set_updated_at
AFTER UPDATE ON users
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
        UPDATE users
        SET updated_at = CURRENT_TIMESTAMP
        WHERE id = NEW.id;
END;

CREATE TRIGGER users_reference_uuid
AFTER INSERT ON users
BEGIN
  UPDATE users
  SET reference_id = (
    select substr(u,1,8)||'-'||
    substr(u,9,4)||'-4'||
    substr(u,13,3)||'-'||v||
    substr(u,17,3)||'-'||
    substr(u,21,12) from (
        select
            lower(hex(randomblob(16))) as u,
            substr('89ab',abs(random()) % 4 + 1, 1) as v)
    )
  WHERE id = NEW.id;
END;

CREATE TABLE magic_token (
    id INTEGER PRIMARY KEY,
    email TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE,
    action TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL DEFAULT (DATETIME('now', '+3 hour'))
);

CREATE TABLE filemanager_files (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- owner of the file
    original_filename TEXT NOT NULL,
    filename TEXT NOT NULL UNIQUE,
    filesize INTEGER NOT NULL, -- in bytes
    filetype TEXT,
    filehash TEXT, -- e.g. SHA256 hash of the file
    filetag TEXT, -- e.g. category or tag for the file (user_avatar, document, etc.)
    filedescription TEXT,
    processed INTEGER NOT NULL DEFAULT 0 CHECK (processed IN (0,1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted INTEGER NOT NULL DEFAULT 0 CHECK (deleted IN (0,1))
);

CREATE INDEX idx_filemanager_files_user_id ON filemanager_files(user_id);
CREATE INDEX idx_filemanager_files_user_id_deleted
    ON filemanager_files(user_id, deleted);
CREATE INDEX idx_filemanager_files_hash ON filemanager_files(filehash);

CREATE TRIGGER filemanager_files_set_updated_at
AFTER UPDATE OF original_filename, filename, filesize, filetype, filehash, filetag, filedescription, processed ON filemanager_files
BEGIN
    UPDATE filemanager_files SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

CREATE TRIGGER filemanager_files_set_updated_at_on_deleted
AFTER UPDATE ON filemanager_files
FOR EACH ROW
WHEN NEW.deleted != OLD.deleted
BEGIN
    UPDATE filemanager_files SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

CREATE VIRTUAL TABLE filemanager_files_fts
USING fts5(
    original_filename,
    filename,
    filetag,
    filedescription,
    content='filemanager_files',
    content_rowid='id'
);

CREATE TRIGGER filemanager_files_ai
AFTER INSERT ON filemanager_files
BEGIN
  INSERT INTO filemanager_files_fts(rowid, original_filename, filename, filetag, filedescription)
    SELECT NEW.id, NEW.original_filename, NEW.filename, NEW.filetag, NEW.filedescription WHERE NEW.deleted = 0;
END;

CREATE TRIGGER filemanager_files_ad
AFTER DELETE ON filemanager_files
BEGIN
  INSERT INTO filemanager_files_fts(filemanager_files_fts, rowid)
    VALUES('delete', OLD.id);
END;

CREATE TRIGGER filemanager_files_au
AFTER UPDATE OF original_filename, filename, filetag, filedescription, deleted ON filemanager_files
BEGIN
  -- remove old index entry
  INSERT INTO filemanager_files_fts(filemanager_files_fts, rowid)
    VALUES('delete', OLD.id);
  -- add new entry only if not deleted
  INSERT INTO filemanager_files_fts(rowid, original_filename, filename, filetag, filedescription)
    SELECT NEW.id, NEW.original_filename, NEW.filename, NEW.filetag, NEW.filedescription WHERE NEW.deleted = 0;
END;

-- 0002_eav.up.sql
-- EAV core (single-tenant, no workspaces, no UI/forms metadata).
-- SQLite-first design: typed value columns, opaque reference_id, optimistic locking via rev.

-- ----------------------------------------------------------------------
-- eav_entity_types
-- Logical schemas (similar to "tables" in Access).
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS eav_entity_types (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT    NOT NULL,
    machine_name  TEXT    NOT NULL COLLATE NOCASE,
    name          TEXT    NOT NULL,
    description   TEXT,
    pre_save      TEXT,  -- Filo script executed before saving records
    pos_load      TEXT,  -- Filo script executed after loading records (before display)
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME
);

-- Partial unique indexes (exclude soft-deleted rows)
CREATE UNIQUE INDEX IF NOT EXISTS idx_eav_entity_types_reference_id_active
    ON eav_entity_types(reference_id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_eav_entity_types_machine_name_active
    ON eav_entity_types(machine_name COLLATE NOCASE) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_eav_entity_types_machine_name_nocase
    ON eav_entity_types(LOWER(machine_name));

CREATE INDEX IF NOT EXISTS idx_eav_entity_types_deleted_at
    ON eav_entity_types(deleted_at);

-- ----------------------------------------------------------------------
-- eav_attributes
-- Logical columns belonging to an entity type.
-- UI metadata is NOT stored here.
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS eav_attributes (
    id             INTEGER PRIMARY KEY,
    reference_id   TEXT    NOT NULL,

    entity_type_id INTEGER NOT NULL
        REFERENCES eav_entity_types(id) ON DELETE CASCADE,

    machine_name   TEXT    NOT NULL COLLATE NOCASE,
    label          TEXT    NOT NULL,
    help_text      TEXT,

    primitive_kind TEXT    NOT NULL CHECK (primitive_kind IN (
        'BOOL', 
        'INT', 
        'REAL', 
        'TEXT', 
        'DATETIME'
    )),

    is_required    INTEGER NOT NULL DEFAULT 0 CHECK (is_required IN (0,1)),
    is_unique      INTEGER NOT NULL DEFAULT 0 CHECK (is_unique IN (0,1)),
    is_indexed     INTEGER NOT NULL DEFAULT 0 CHECK (is_indexed IN (0,1)),
    max_length     INTEGER DEFAULT 256,  -- For TEXT fields, max character count

    is_computed    INTEGER NOT NULL DEFAULT 0 CHECK (is_computed IN (0,1)),
    computed_expr  TEXT,

    -- Default values for new records (user-defined)
    -- Exactly one should be set based on primitive_kind
    default_v_bool     INTEGER CHECK (default_v_bool IN (0,1)),
    default_v_int      INTEGER,
    default_v_real     REAL,
    default_v_text     TEXT,
    default_v_datetime DATETIME,

    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at     DATETIME
);

-- Partial unique indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_eav_attributes_reference_id_active
    ON eav_attributes(reference_id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_eav_attributes_entity_type_machine_name_active
    ON eav_attributes(entity_type_id, machine_name COLLATE NOCASE) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_eav_attributes_entity_type_id
    ON eav_attributes(entity_type_id);

CREATE INDEX IF NOT EXISTS idx_eav_attributes_entity_type_machine_name
    ON eav_attributes(entity_type_id, LOWER(machine_name));

CREATE INDEX IF NOT EXISTS idx_eav_attributes_deleted_at
    ON eav_attributes(deleted_at);

-- ----------------------------------------------------------------------
-- eav_records
-- Instances (rows) of a given entity type.
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS eav_records (
    id             INTEGER PRIMARY KEY,
    reference_id   TEXT     NOT NULL,

    entity_type_id INTEGER  NOT NULL
        REFERENCES eav_entity_types(id) ON DELETE CASCADE,

    status         TEXT     NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active')),
    rev            INTEGER  NOT NULL DEFAULT 1 CHECK (rev >= 1),

    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at     DATETIME
);

-- Partial unique index
CREATE UNIQUE INDEX IF NOT EXISTS idx_eav_records_reference_id_active
    ON eav_records(reference_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_eav_records_entity_type_id
    ON eav_records(entity_type_id);

CREATE INDEX IF NOT EXISTS idx_eav_records_entity_type_updated_at
    ON eav_records(entity_type_id, updated_at);

CREATE INDEX IF NOT EXISTS idx_eav_records_deleted_at
    ON eav_records(deleted_at);

CREATE INDEX IF NOT EXISTS idx_eav_records_status
    ON eav_records(status) WHERE deleted_at IS NULL;

-- ----------------------------------------------------------------------
-- eav_values
-- Typed cell storage per (record, attribute).
-- Exactly one v_* column must be non-NULL.
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS eav_values (
    record_id     INTEGER NOT NULL
        REFERENCES eav_records(id) ON DELETE CASCADE,

    attribute_id  INTEGER NOT NULL
        REFERENCES eav_attributes(id) ON DELETE CASCADE,

    v_bool        INTEGER CHECK (v_bool IN (0,1)),
    v_int         INTEGER,
    v_real        REAL,
    v_text        TEXT,
    v_datetime    DATETIME, -- ISO-8601 UTC text

    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (record_id, attribute_id),

    -- Enforce exactly one value column set.
    CHECK (
        (v_bool     IS NOT NULL) +
        (v_int      IS NOT NULL) +
        (v_real     IS NOT NULL) +
        (v_text     IS NOT NULL) +
        (v_datetime IS NOT NULL)
        = 1
    )
);

CREATE INDEX IF NOT EXISTS idx_eav_values_record_id
    ON eav_values(record_id);

CREATE INDEX IF NOT EXISTS idx_eav_values_attribute_id
    ON eav_values(attribute_id);

-- Per-type indexes (created now for predictable performance on filters/sorts).
CREATE INDEX IF NOT EXISTS idx_eav_values_attr_v_int
    ON eav_values(attribute_id, v_int);

CREATE INDEX IF NOT EXISTS idx_eav_values_attr_v_real
    ON eav_values(attribute_id, v_real);

CREATE INDEX IF NOT EXISTS idx_eav_values_attr_v_datetime
    ON eav_values(attribute_id, v_datetime);

CREATE INDEX IF NOT EXISTS idx_eav_values_attr_v_text_nocase
    ON eav_values(attribute_id, v_text COLLATE NOCASE);

-- ----------------------------------------------------------------------
-- Triggers for Optimistic Locking
-- ----------------------------------------------------------------------

-- Trigger to enforce optimistic locking on eav_records updates.
-- This trigger validates that the rev being updated matches the current value.
-- Applications must pass the current rev in the WHERE clause of their UPDATE.
-- If rev doesn't match, the UPDATE affects 0 rows, signaling a conflict.
--
-- Note: SQLite doesn't support BEFORE UPDATE triggers that can abort based on
-- conditions in a clean way, so we rely on the UPDATE affecting 0 rows when
-- the WHERE clause (including rev check) doesn't match any record.
--
-- The trigger below ensures rev is always incremented and updated_at is refreshed.

CREATE TRIGGER IF NOT EXISTS trg_eav_records_update_rev
AFTER UPDATE ON eav_records
FOR EACH ROW
WHEN OLD.deleted_at IS NULL AND NEW.deleted_at IS NULL
BEGIN
    -- Ensure rev was incremented by exactly 1
    SELECT CASE
        WHEN NEW.rev != OLD.rev + 1 THEN
            RAISE(ABORT, 'eav_records: rev must be incremented by exactly 1')
    END;
END;

-- ----------------------------------------------------------------------
-- Notes:
-- - Type matching between attribute.primitive_kind and the chosen v_* column
--   is enforced in Go code (cannot be enforced by SQLite CHECK without joins).
-- - Uniqueness of attribute values (when is_unique=1) is enforced in Go code
--   (or via future specialized indexes per primitive kind if you choose).
-- - Forms/UI plugins are defined in separate tables and migrations (not here).
-- ----------------------------------------------------------------------

-- 0005_menus.up.sql
-- Menus / Navigation layer (single-tenant).
-- Menus are reusable navigation components that can be assigned to forms.
-- Menu items are recursive: any item can contain other items via parent_id.

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

-- 0003_forms.up.sql
-- Forms / UI projection layer (single-tenant).
-- Forms are UI projections over data sources (EAV entity types or relational tables).
-- A form does not own data; it references a data source.
-- Elements are recursive: any element can contain other elements via parent_id.

-- ----------------------------------------------------------------------
-- forms
-- A form is a UI projection over an EAV entity type (or relational table).
-- It does not own data. The root data source determines the primary record.
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forms (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT    NOT NULL,

    machine_name  TEXT    NOT NULL COLLATE NOCASE, -- stable key for routing
    label         TEXT    NOT NULL,
    description   TEXT,

    -- Data source binding (simplified: directly reference EAV entity type)
    -- For relational tables, use eav_entity_type_id = NULL and store table_name
    eav_entity_type_id INTEGER
        REFERENCES eav_entity_types(id) ON DELETE SET NULL,

    -- When true, form does not display the submit button (Criar/Salvar)
    hide_submit_button  INTEGER NOT NULL DEFAULT 0 CHECK (hide_submit_button IN (0, 1)),
    -- When true, form does not display the cancel button
    hide_cancel_button  INTEGER NOT NULL DEFAULT 0 CHECK (hide_cancel_button IN (0, 1)),
    
    -- When true, form does not display the title header
    hide_title          INTEGER NOT NULL DEFAULT 0 CHECK (hide_title IN (0, 1)),

    -- Debug/System Info visibility
    show_system_info    INTEGER NOT NULL DEFAULT 0 CHECK (show_system_info IN (0, 1)),

    -- Associated menu for navbar display when form is active
    menu_id INTEGER REFERENCES menus(id) ON DELETE SET NULL,

    -- Search form: /form/{name} opens the record listing (search view)
    -- instead of the create view. Several search forms may point at the
    -- same entity type, each exposing different columns.
    is_search INTEGER NOT NULL DEFAULT 0 CHECK (is_search IN (0, 1)),

    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME
);

-- Partial unique indexes (exclude soft-deleted rows)
CREATE UNIQUE INDEX IF NOT EXISTS idx_forms_reference_id_active
    ON forms(reference_id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_forms_machine_name_active
    ON forms(machine_name COLLATE NOCASE) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_forms_deleted_at
    ON forms(deleted_at);

-- Trigger for reference_id generation
CREATE TRIGGER IF NOT EXISTS trg_forms_reference_id
AFTER INSERT ON forms
WHEN NEW.reference_id IS NULL OR NEW.reference_id = ''
BEGIN
    UPDATE forms 
    SET reference_id = lower(hex(randomblob(24)))
    WHERE id = NEW.id;
END;

-- ----------------------------------------------------------------------
-- form_elements
-- Unified table for all form UI elements: groups, fields, dividers, etc.
-- Elements are recursive via parent_id (a group can contain other elements).
-- Elements bind to EAV attributes or are UI-only.
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS form_elements (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT    NOT NULL,

    form_id       INTEGER NOT NULL
        REFERENCES forms(id) ON DELETE CASCADE,

    -- Recursive: parent element (NULL = root level)
    parent_id     INTEGER
        REFERENCES form_elements(id) ON DELETE CASCADE,

    -- Stable identifier within the form
    machine_name  TEXT    NOT NULL COLLATE NOCASE,

    -- Element type: 'group', 'field', 'divider', 'accordion', etc.
    element_kind  TEXT    NOT NULL,

    -- Display
    label         TEXT,
    help_text     TEXT,

    -- Ordering within parent
    z_order       INTEGER NOT NULL DEFAULT 0,

    -- Bootstrap grid column span (1-12, default 12 = full width)
    col_span      INTEGER NOT NULL DEFAULT 12 CHECK (col_span >= 1 AND col_span <= 12),

    -- Horizontal alignment when element is smaller than available columns
    -- 'left' (default), 'center', 'right'
    alignment     TEXT NOT NULL DEFAULT 'left' CHECK (alignment IN ('left', 'center', 'right')),

    -- UI plugin ID for rendering (e.g., 'text', 'textarea', 'date_picker')
    -- For groups this might be 'group', 'accordion', 'tabs', etc.
    ui_kind       TEXT,
    ui_meta_json  TEXT,  -- Plugin-specific options

    -- Binding to EAV attribute (NULL for UI-only elements)
    eav_attribute_id INTEGER
        REFERENCES eav_attributes(id) ON DELETE RESTRICT,

    -- UI-only element (no persistence, like dividers)
    is_ui_only    INTEGER NOT NULL DEFAULT 0 CHECK (is_ui_only IN (0, 1)),

    -- Read-only at form level (independent of EAV attribute)
    is_readonly   INTEGER NOT NULL DEFAULT 0 CHECK (is_readonly IN (0, 1)),

    -- Hide label and help text display options
    hide_label    INTEGER NOT NULL DEFAULT 0 CHECK (hide_label IN (0, 1)),
    hide_help_text INTEGER NOT NULL DEFAULT 0 CHECK (hide_help_text IN (0, 1)),

    -- Optional Filo expressions
    validate_expr  TEXT,
    computed_expr  TEXT,

    -- Button-specific properties (only used when element_kind = 'button')
    button_filo_code   TEXT,      -- Server-side Filo script (never sent to client)
    button_run_save    INTEGER NOT NULL DEFAULT 0 CHECK (button_run_save IN (0, 1)),
    button_js_code     TEXT,      -- Client-side JavaScript
    button_style       TEXT DEFAULT 'primary',  -- Bootstrap button style
    button_confirm_msg TEXT,      -- Confirmation dialog text

    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME,

    -- Unique machine_name per form
    UNIQUE(form_id, machine_name)
);

-- Partial unique indexes (exclude soft-deleted rows)
CREATE UNIQUE INDEX IF NOT EXISTS idx_form_elements_reference_id_active
    ON form_elements(reference_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_form_elements_form_order
    ON form_elements(form_id, parent_id, z_order, id);

CREATE INDEX IF NOT EXISTS idx_form_elements_parent_order
    ON form_elements(parent_id, z_order, id);

CREATE INDEX IF NOT EXISTS idx_form_elements_deleted_at
    ON form_elements(deleted_at);

-- Trigger for reference_id generation
CREATE TRIGGER IF NOT EXISTS trg_form_elements_reference_id
AFTER INSERT ON form_elements
WHEN NEW.reference_id IS NULL OR NEW.reference_id = ''
BEGIN
    UPDATE form_elements 
    SET reference_id = lower(hex(randomblob(24)))
    WHERE id = NEW.id;
END;

-- ----------------------------------------------------------------------
-- Notes:
-- - This migration intentionally does not define permissions tables.
--   Apps own access control.
-- - ui_meta_json is validated in Go (not by SQL).
-- - EAV attribute constraints (required, max_length, etc.) are enforced
--   by respecting the bound eav_attributes metadata.
-- - form_elements is recursive: parent_id points to containing element.
--   The rendering engine traverses the tree to build the form layout.
-- ----------------------------------------------------------------------

-- v_eav_cells
-- Generic EAV view exposing cells as a relational stream.
-- Safe to use as a base for reports, pivots and lookups.

CREATE VIEW IF NOT EXISTS v_eav_cells AS
SELECT
    -- Entity type
    et.id            AS entity_type_id,
    et.reference_id  AS entity_ref,
    et.machine_name  AS entity_machine,

    -- Record
    r.id             AS record_id,
    r.reference_id   AS record_ref,
    r.rev            AS record_rev,
    r.created_at     AS record_created_at,
    r.updated_at     AS record_updated_at,

    -- Attribute
    a.id             AS attribute_id,
    a.reference_id   AS attribute_ref,
    a.machine_name   AS attr_machine,
    a.primitive_kind AS attr_kind,

    -- Typed values
    v.v_bool,
    v.v_int,
    v.v_real,
    v.v_text,
    v.v_datetime,

    -- Value metadata
    v.updated_at     AS value_updated_at

FROM eav_records r
JOIN eav_entity_types et
    ON et.id = r.entity_type_id
JOIN eav_values v
    ON v.record_id = r.id
JOIN eav_attributes a
    ON a.id = v.attribute_id

WHERE
    r.deleted_at  IS NULL
    AND et.deleted_at IS NULL
    AND a.deleted_at  IS NULL;

-- 0008_i18n_overrides.up.sql
-- User adjustments to UI translations, layered over the built-in
-- dictionaries at boot. The key IS the US English source string.

CREATE TABLE IF NOT EXISTS i18n_overrides (
    id          INTEGER PRIMARY KEY,
    locale      TEXT NOT NULL,
    msg_key     TEXT NOT NULL,
    translation TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (locale, msg_key)
);

-- 0009_content_translations.up.sql
-- Translations for user-created content (form labels, element labels and
-- help text, attribute labels, menu item labels). ref_id is the object's
-- reference_id; field names the translated column. Applied at render time
-- on runtime screens, falling back to the original text.

CREATE TABLE IF NOT EXISTS content_translations (
    id         INTEGER PRIMARY KEY,
    locale     TEXT NOT NULL,
    ref_id     TEXT NOT NULL,
    field      TEXT NOT NULL,
    text       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (locale, ref_id, field)
);
