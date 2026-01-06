-- 0003_forms.up.sql
-- Forms / UI projection layer (single-tenant).
-- Forms are UI projections over data sources (EAV entity types or relational tables).
-- A form does not own data; it references a data source.
-- Elements are recursive: any element can contain other elements via parent_id.

PRAGMA foreign_keys = ON;

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

    -- Optional Filo expressions
    validate_expr  TEXT,
    computed_expr  TEXT,

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
