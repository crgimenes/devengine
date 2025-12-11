-- Workspaces plus Entity-Attribute-Value schema

CREATE TABLE workspaces (
    id            INTEGER PRIMARY KEY,
    reference_id  TEXT UNIQUE,                -- opaque external ID (e.g., UUIDv7/ULID)
    name          TEXT NOT NULL,
    description   TEXT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_workspaces_name
    ON workspaces(name);

CREATE TABLE eav_forms (
    id             INTEGER PRIMARY KEY,
    reference_id   TEXT UNIQUE,               -- opaque external ID
    workspace_id   INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    owner_user_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    machine_name   TEXT NOT NULL,             -- stable per-workspace identifier
    label          TEXT NOT NULL,             -- display name
    active         INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1)),
    default_view_mode TEXT NOT NULL DEFAULT 'list' CHECK (default_view_mode IN ('list', 'card', 'carousel')),
    card_cols      INTEGER NOT NULL DEFAULT 6 CHECK (card_cols BETWEEN 1 AND 12),
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (workspace_id, machine_name)
);

CREATE INDEX idx_eav_forms_workspace
    ON eav_forms(workspace_id);
CREATE INDEX idx_eav_forms_owner
    ON eav_forms(owner_user_id);

CREATE TABLE eav_records (
    id               INTEGER PRIMARY KEY,
    reference_id     TEXT UNIQUE,             -- opaque external ID
    form_id          INTEGER NOT NULL REFERENCES eav_forms(id) ON DELETE CASCADE,
    workspace_id     INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    owner_user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    tags_json        TEXT,                    -- optional JSON tags

    rev              INTEGER NOT NULL DEFAULT 1, -- optimistic locking
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at       DATETIME                  -- NULL means not deleted
);

CREATE INDEX idx_eav_records_form_updated
    ON eav_records(form_id, updated_at);
CREATE INDEX idx_eav_records_workspace_form
    ON eav_records(workspace_id, form_id, updated_at);
CREATE INDEX idx_eav_records_owner
    ON eav_records(owner_user_id);

CREATE TABLE eav_fields (
    id               INTEGER PRIMARY KEY,
    form_id          INTEGER NOT NULL REFERENCES eav_forms(id) ON DELETE CASCADE,

    machine_name     TEXT NOT NULL,
    label            TEXT NOT NULL,

    z_order          INTEGER NOT NULL DEFAULT 0,
    visible          INTEGER NOT NULL DEFAULT 1 CHECK (visible IN (0,1)),

    is_ui            INTEGER NOT NULL DEFAULT 0 CHECK (is_ui IN (0,1)),
    ui_role          TEXT,

    primitive_kind   TEXT NOT NULL CHECK (primitive_kind IN (
        '-',
        'BOOL',
        'DATETIME',
        'FLOAT',
        'INT',
        'TEXT'
    )),
    ui_kind          TEXT NOT NULL,
    ui_meta_json     TEXT,

    is_readonly      INTEGER NOT NULL DEFAULT 0 CHECK (is_readonly IN (0,1)),
    required         INTEGER NOT NULL DEFAULT 0 CHECK (required IN (0,1)),

    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expression       TEXT,
    expression_order INTEGER,
    column_width     INTEGER NOT NULL DEFAULT 12 CHECK (column_width BETWEEN 1 AND 12),
    parent_group_machine_name TEXT,
    is_grouping_field INTEGER NOT NULL DEFAULT 0,

    list_visible     INTEGER NOT NULL DEFAULT 1 CHECK (list_visible IN (0,1)),
    list_z_order     INTEGER NOT NULL DEFAULT 0,
    list_cols        INTEGER NOT NULL DEFAULT 12 CHECK (list_cols BETWEEN 1 AND 12),

    card_visible     INTEGER NOT NULL DEFAULT 1 CHECK (card_visible IN (0,1)),
    card_z_order     INTEGER NOT NULL DEFAULT 0,
    card_cols        INTEGER NOT NULL DEFAULT 12 CHECK (card_cols BETWEEN 1 AND 12),

    carousel_visible INTEGER NOT NULL DEFAULT 1 CHECK (carousel_visible IN (0,1)),
    carousel_z_order INTEGER NOT NULL DEFAULT 0,
    carousel_cols    INTEGER NOT NULL DEFAULT 12 CHECK (carousel_cols BETWEEN 1 AND 12),

    fts_index        INTEGER NOT NULL DEFAULT 0 CHECK (fts_index IN (0,1)),

    UNIQUE (form_id, machine_name)
);

CREATE INDEX idx_eav_fields_order
    ON eav_fields(form_id, z_order, label);
CREATE INDEX idx_eav_fields_form_machine
    ON eav_fields(form_id, machine_name);

CREATE TABLE eav_values (
    record_id      INTEGER NOT NULL REFERENCES eav_records(id) ON DELETE CASCADE,
    field_id       INTEGER NOT NULL REFERENCES eav_fields(id)  ON DELETE CASCADE,
    form_id        INTEGER NOT NULL REFERENCES eav_forms(id)   ON DELETE CASCADE,

    value_bool     INTEGER CHECK (value_bool IN (0,1)),
    value_datetime DATETIME,
    value_float    REAL,
    value_int      INTEGER,
    value_text     TEXT,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (record_id, field_id)
);

CREATE INDEX idx_eav_values_record
    ON eav_values(record_id);

CREATE UNIQUE INDEX idx_eav_forms_machine_name_unique
    ON eav_forms(machine_name);

-- Trigger to update eav_records.updated_at when eav_values is modified
CREATE TRIGGER trg_eav_values_update_record_timestamp
AFTER UPDATE ON eav_values
FOR EACH ROW
BEGIN
    UPDATE eav_records 
    SET updated_at = CURRENT_TIMESTAMP 
    WHERE id = NEW.record_id;
END;

-- =============================================================================
-- FTS5 Full-Text Search for EAV Records
-- =============================================================================
-- Only TEXT fields with fts_index=1 are indexed.
-- The FTS table stores searchable content with references to record/field/form.

CREATE VIRTUAL TABLE eav_fts USING fts5(
    content,                    -- searchable text from value_text
    record_id UNINDEXED,        -- for filtering (not searchable)
    field_id UNINDEXED,         -- for filtering (not searchable)
    form_id UNINDEXED           -- for filtering (not searchable)
);

-- Trigger: Insert into FTS when a value is created for an FTS-indexed TEXT field
CREATE TRIGGER trg_eav_fts_insert
AFTER INSERT ON eav_values
WHEN NEW.value_text IS NOT NULL
BEGIN
    INSERT INTO eav_fts(content, record_id, field_id, form_id)
    SELECT NEW.value_text, NEW.record_id, NEW.field_id, NEW.form_id
    FROM eav_fields f
    WHERE f.id = NEW.field_id
      AND f.primitive_kind = 'TEXT'
      AND f.fts_index = 1;
END;

-- Trigger: Update FTS when value_text changes
CREATE TRIGGER trg_eav_fts_update
AFTER UPDATE OF value_text ON eav_values
WHEN NEW.value_text IS NOT NULL OR OLD.value_text IS NOT NULL
BEGIN
    -- Remove old entry
    DELETE FROM eav_fts
    WHERE record_id = OLD.record_id AND field_id = OLD.field_id;
    
    -- Insert new entry only if field is FTS indexed and new value exists
    INSERT INTO eav_fts(content, record_id, field_id, form_id)
    SELECT NEW.value_text, NEW.record_id, NEW.field_id, NEW.form_id
    FROM eav_fields f
    WHERE f.id = NEW.field_id
      AND f.primitive_kind = 'TEXT'
      AND f.fts_index = 1
      AND NEW.value_text IS NOT NULL;
END;

-- Trigger: Delete from FTS when a value row is deleted
CREATE TRIGGER trg_eav_fts_delete
AFTER DELETE ON eav_values
BEGIN
    DELETE FROM eav_fts
    WHERE record_id = OLD.record_id AND field_id = OLD.field_id;
END;

-- Trigger: Remove from FTS when a record is soft-deleted
CREATE TRIGGER trg_eav_fts_record_soft_delete
AFTER UPDATE OF deleted_at ON eav_records
WHEN NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL
BEGIN
    DELETE FROM eav_fts WHERE record_id = NEW.id;
END;

-- Trigger: Restore to FTS when a record is un-deleted
CREATE TRIGGER trg_eav_fts_record_restore
AFTER UPDATE OF deleted_at ON eav_records
WHEN NEW.deleted_at IS NULL AND OLD.deleted_at IS NOT NULL
BEGIN
    INSERT INTO eav_fts(content, record_id, field_id, form_id)
    SELECT v.value_text, v.record_id, v.field_id, v.form_id
    FROM eav_values v
    JOIN eav_fields f ON f.id = v.field_id
    WHERE v.record_id = NEW.id
      AND f.primitive_kind = 'TEXT'
      AND f.fts_index = 1
      AND v.value_text IS NOT NULL;
END;

-- Trigger: Remove field entries from FTS when fts_index is turned off
CREATE TRIGGER trg_eav_fts_field_index_off
AFTER UPDATE OF fts_index ON eav_fields
WHEN NEW.fts_index = 0 AND OLD.fts_index = 1
BEGIN
    DELETE FROM eav_fts WHERE field_id = NEW.id;
END;

-- Trigger: Add field entries to FTS when fts_index is turned on
CREATE TRIGGER trg_eav_fts_field_index_on
AFTER UPDATE OF fts_index ON eav_fields
WHEN NEW.fts_index = 1 AND OLD.fts_index = 0 AND NEW.primitive_kind = 'TEXT'
BEGIN
    INSERT INTO eav_fts(content, record_id, field_id, form_id)
    SELECT v.value_text, v.record_id, v.field_id, v.form_id
    FROM eav_values v
    JOIN eav_records r ON r.id = v.record_id
    WHERE v.field_id = NEW.id
      AND v.value_text IS NOT NULL
      AND r.deleted_at IS NULL;
END;
