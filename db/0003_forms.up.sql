-- 0003_forms.up.sql
-- Forms / UI projection layer (single-tenant).
-- This schema describes forms, blocks, and fields, and binds them to data sources
-- (EAV attributes or relational columns). No permissions, no navigation flows,
-- no workspaces. Apps decide who can edit this.

PRAGMA foreign_keys = ON;

-- ----------------------------------------------------------------------
-- form_data_sources
-- Abstract definition of a data source the form can read/write.
-- It can point to:
--   - EAV entity type (stored in eav_entity_types)
--   - Relational table (declared by the app for discoverability)
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS form_data_sources (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT    NOT NULL UNIQUE, -- opaque external identifier

    kind          TEXT    NOT NULL CHECK (kind IN ('eav', 'rel')),
    -- Stable key used by the app, for example:
    --   eav: machine_name of eav_entity_types
    --   rel: SQL table name (or an app-defined key)
    source_key    TEXT    NOT NULL,

    label         TEXT    NOT NULL,
    description   TEXT,

    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME,

    UNIQUE(kind, source_key)
);

CREATE INDEX IF NOT EXISTS idx_form_data_sources_kind_key
    ON form_data_sources(kind, source_key);

CREATE INDEX IF NOT EXISTS idx_form_data_sources_deleted_at
    ON form_data_sources(deleted_at);

-- ----------------------------------------------------------------------
-- forms
-- A form is a UI projection of a primary data source.
-- It does not own data. It references one data source as the "root".
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forms (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT    NOT NULL UNIQUE, -- opaque external identifier

    machine_name  TEXT    NOT NULL UNIQUE COLLATE NOCASE, -- stable key for routing
    label         TEXT    NOT NULL,
    description   TEXT,

    root_data_source_id INTEGER NOT NULL
        REFERENCES form_data_sources(id) ON DELETE RESTRICT,

    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_forms_machine_name_nocase
    ON forms(LOWER(machine_name));

CREATE INDEX IF NOT EXISTS idx_forms_root_data_source_id
    ON forms(root_data_source_id);

CREATE INDEX IF NOT EXISTS idx_forms_deleted_at
    ON forms(deleted_at);

-- ----------------------------------------------------------------------
-- form_blocks
-- Blocks allow building "Access-like" forms:
--   - fields block
--   - list block
--   - subform block
--   - attachments block (integrates with files module)
--   - computed panel block (Filo)
-- Blocks are ordered within a form.
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS form_blocks (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT    NOT NULL UNIQUE, -- opaque external identifier

    form_id       INTEGER NOT NULL
        REFERENCES forms(id) ON DELETE CASCADE,

    kind          TEXT    NOT NULL CHECK (kind IN (
        'fields', 'list', 'subform', 'attachments', 'computed_panel', 'custom'
    )),

    label         TEXT,
    help_text     TEXT,

    z_order       INTEGER NOT NULL DEFAULT 0,

    -- Optional: a block can target a different data source (e.g., related list).
    data_source_id INTEGER
        REFERENCES form_data_sources(id) ON DELETE RESTRICT,

    -- Optional: JSON config for block behavior (validated by Go).
    meta_json     TEXT,

    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_form_blocks_form_order
    ON form_blocks(form_id, z_order, id);

CREATE INDEX IF NOT EXISTS idx_form_blocks_deleted_at
    ON form_blocks(deleted_at);

-- ----------------------------------------------------------------------
-- form_fields
-- A field is a UI element that may be bound to:
--   - an EAV attribute, or
--   - a relational column, or
--   - no persistence (UI-only).
-- Plugins define how the field is rendered and parsed.
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS form_fields (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT    NOT NULL UNIQUE, -- opaque external identifier

    form_id       INTEGER NOT NULL
        REFERENCES forms(id) ON DELETE CASCADE,

    block_id      INTEGER
        REFERENCES form_blocks(id) ON DELETE SET NULL,

    -- Stable identifier within the form (useful for scripts, test fixtures, etc.)
    machine_name  TEXT    NOT NULL COLLATE NOCASE,

    label         TEXT,
    help_text     TEXT,

    z_order       INTEGER NOT NULL DEFAULT 0,

    -- Plugin ID, e.g. "text", "textarea", "divider", "rpg.hpbar"
    ui_kind       TEXT    NOT NULL,
    ui_meta_json  TEXT,

    -- UI-only element (no persistence).
    is_ui_only    INTEGER NOT NULL DEFAULT 0 CHECK (is_ui_only IN (0,1)),

    -- Read-only behavior at the form level (independent of storage).
    is_readonly   INTEGER NOT NULL DEFAULT 0 CHECK (is_readonly IN (0,1)),

    -- Binding (exactly one when is_ui_only = 0).
    bind_kind     TEXT    NOT NULL DEFAULT 'none'
        CHECK (bind_kind IN ('none', 'eav', 'rel')),

    bind_eav_attribute_id INTEGER
        REFERENCES eav_attributes(id) ON DELETE RESTRICT,

    -- For bind_kind='rel', the field name is stored directly (no FK)
    bind_rel_table_name TEXT,
    bind_rel_column_name TEXT,

    -- Optional expression hooks (Filo), stored as text.
    validate_expr  TEXT,
    computed_expr  TEXT,
    expression_order INTEGER NOT NULL DEFAULT 0,

    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME,

    UNIQUE(form_id, machine_name),

    -- Enforce binding consistency.
    CHECK (
        (is_ui_only = 1 AND bind_kind = 'none' AND bind_eav_attribute_id IS NULL AND bind_rel_table_name IS NULL AND bind_rel_column_name IS NULL)
        OR
        (is_ui_only = 0 AND bind_kind = 'eav'  AND bind_eav_attribute_id IS NOT NULL AND bind_rel_table_name IS NULL AND bind_rel_column_name IS NULL)
        OR
        (is_ui_only = 0 AND bind_kind = 'rel'  AND bind_eav_attribute_id IS NULL AND bind_rel_table_name IS NOT NULL AND bind_rel_column_name IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_form_fields_form_order
    ON form_fields(form_id, z_order, id);

CREATE INDEX IF NOT EXISTS idx_form_fields_form_machine_name
    ON form_fields(form_id, LOWER(machine_name));

CREATE INDEX IF NOT EXISTS idx_form_fields_block_order
    ON form_fields(block_id, z_order, id);

CREATE INDEX IF NOT EXISTS idx_form_fields_ui_kind
    ON form_fields(ui_kind);

CREATE INDEX IF NOT EXISTS idx_form_fields_deleted_at
    ON form_fields(deleted_at);

-- ----------------------------------------------------------------------
-- Notes:
-- - This migration intentionally does not define permissions tables.
--   Apps own access control.
-- - ui_meta_json and meta_json are validated in Go (not by SQL).
-- - EAV attribute primitive_kind compatibility with plugin behavior is enforced
--   in Go (a plugin may support multiple primitive kinds).
-- - form_data_sources is optional but recommended as a stable abstraction so
--   forms can reference either EAV or relational sources uniformly.
-- ----------------------------------------------------------------------

