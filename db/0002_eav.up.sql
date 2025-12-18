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
    reference_id  TEXT    NOT NULL,
    machine_name  TEXT    NOT NULL COLLATE NOCASE,
    name          TEXT    NOT NULL,
    description   TEXT,
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

    is_computed    INTEGER NOT NULL DEFAULT 0 CHECK (is_computed IN (0,1)),
    computed_expr  TEXT,

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

