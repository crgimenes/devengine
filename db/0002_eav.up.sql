-- 0002_eav.up.sql
-- EAV core (single-tenant, no workspaces, no UI/forms metadata).
-- SQLite-first design: typed value columns, opaque reference_id, optimistic locking via rev.

PRAGMA foreign_keys = ON;

-- ----------------------------------------------------------------------
-- eav_entity_types
-- Logical schemas (similar to "tables" in Access).
-- ----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS eav_entity_types (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT    NOT NULL UNIQUE, -- opaque external identifier
    machine_name  TEXT    NOT NULL UNIQUE COLLATE NOCASE, -- stable identifier for code/routes
    name          TEXT    NOT NULL, -- human label
    description   TEXT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME
);

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
    reference_id   TEXT    NOT NULL UNIQUE, -- opaque external identifier

    entity_type_id INTEGER NOT NULL
        REFERENCES eav_entity_types(id) ON DELETE CASCADE,

    machine_name   TEXT    NOT NULL COLLATE NOCASE, -- stable identifier within an entity type
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

    -- Expression support is stored, not necessarily executed in MVP.
    -- These are Filo snippets evaluated by integration code (not by SQL).
    is_computed    INTEGER NOT NULL DEFAULT 0 CHECK (is_computed IN (0,1)),
    default_expr   TEXT, -- optional Filo snippet
    computed_expr  TEXT, -- optional Filo snippet

    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at     DATETIME,

    UNIQUE(entity_type_id, machine_name)
);

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
    reference_id   TEXT     NOT NULL UNIQUE, -- opaque external identifier

    entity_type_id INTEGER  NOT NULL
        REFERENCES eav_entity_types(id) ON DELETE CASCADE,

    -- Optimistic concurrency control.
    rev            INTEGER  NOT NULL DEFAULT 1 CHECK (rev >= 1),

    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at     DATETIME
);

CREATE INDEX IF NOT EXISTS idx_eav_records_entity_type_id
    ON eav_records(entity_type_id);

CREATE INDEX IF NOT EXISTS idx_eav_records_entity_type_updated_at
    ON eav_records(entity_type_id, updated_at);

CREATE INDEX IF NOT EXISTS idx_eav_records_deleted_at
    ON eav_records(deleted_at);

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
-- If you later decide to delay these until metrics justify, remove them.
CREATE INDEX IF NOT EXISTS idx_eav_values_attr_v_int
    ON eav_values(attribute_id, v_int);

CREATE INDEX IF NOT EXISTS idx_eav_values_attr_v_real
    ON eav_values(attribute_id, v_real);

CREATE INDEX IF NOT EXISTS idx_eav_values_attr_v_datetime
    ON eav_values(attribute_id, v_datetime);

CREATE INDEX IF NOT EXISTS idx_eav_values_attr_v_text_nocase
    ON eav_values(attribute_id, v_text COLLATE NOCASE);

-- ----------------------------------------------------------------------
-- Notes:
-- - Type matching between attribute.primitive_kind and the chosen v_* column
--   is enforced in Go code (cannot be enforced by SQLite CHECK without joins).
-- - Uniqueness of attribute values (when is_unique=1) is enforced in Go code
--   (or via future specialized indexes per primitive kind if you choose).
-- - Forms/UI plugins are defined in separate tables and migrations (not here).
-- ----------------------------------------------------------------------

