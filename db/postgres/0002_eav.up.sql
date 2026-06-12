-- EAV core (PostgreSQL dialect): typed value columns, opaque reference_id,
-- optimistic locking via rev. Case-insensitive machine_name lookups use
-- LOWER() expression indexes (the SQLite version relies on COLLATE NOCASE).

CREATE TABLE eav_entity_types (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reference_id  TEXT NOT NULL,
    machine_name  TEXT NOT NULL,
    name          TEXT NOT NULL,
    description   TEXT,
    pre_save      TEXT,  -- Filo script executed before saving records
    pos_load      TEXT,  -- Filo script executed after loading records
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_eav_entity_types_reference_id_active
    ON eav_entity_types(reference_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_eav_entity_types_machine_name_active
    ON eav_entity_types(LOWER(machine_name)) WHERE deleted_at IS NULL;
CREATE INDEX idx_eav_entity_types_deleted_at ON eav_entity_types(deleted_at);

CREATE TRIGGER eav_entity_types_set_updated_at
    BEFORE UPDATE ON eav_entity_types
    FOR EACH ROW EXECUTE FUNCTION devengine_set_updated_at();

CREATE TABLE eav_attributes (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reference_id   TEXT NOT NULL,

    entity_type_id BIGINT NOT NULL
        REFERENCES eav_entity_types(id) ON DELETE CASCADE,

    machine_name   TEXT NOT NULL,
    label          TEXT NOT NULL,
    help_text      TEXT,

    primitive_kind TEXT NOT NULL CHECK (primitive_kind IN (
        'BOOL',
        'INT',
        'REAL',
        'TEXT',
        'DATETIME'
    )),

    is_required    BOOLEAN NOT NULL DEFAULT FALSE,
    is_unique      BOOLEAN NOT NULL DEFAULT FALSE,
    is_indexed     BOOLEAN NOT NULL DEFAULT FALSE,
    max_length     INTEGER DEFAULT 256,

    is_computed    BOOLEAN NOT NULL DEFAULT FALSE,
    computed_expr  TEXT,

    -- Default values for new records; exactly one is set per primitive_kind.
    default_v_bool     BOOLEAN,
    default_v_int      BIGINT,
    default_v_real     DOUBLE PRECISION,
    default_v_text     TEXT,
    default_v_datetime TEXT, -- ISO-8601 text, same as v_datetime

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_eav_attributes_reference_id_active
    ON eav_attributes(reference_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_eav_attributes_entity_type_machine_name_active
    ON eav_attributes(entity_type_id, LOWER(machine_name)) WHERE deleted_at IS NULL;
CREATE INDEX idx_eav_attributes_entity_type_id ON eav_attributes(entity_type_id);
CREATE INDEX idx_eav_attributes_deleted_at ON eav_attributes(deleted_at);

CREATE TRIGGER eav_attributes_set_updated_at
    BEFORE UPDATE ON eav_attributes
    FOR EACH ROW EXECUTE FUNCTION devengine_set_updated_at();

CREATE TABLE eav_records (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reference_id   TEXT NOT NULL,

    entity_type_id BIGINT NOT NULL
        REFERENCES eav_entity_types(id) ON DELETE CASCADE,

    status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active')),
    rev            INTEGER NOT NULL DEFAULT 1 CHECK (rev >= 1),

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_eav_records_reference_id_active
    ON eav_records(reference_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_eav_records_entity_type_id ON eav_records(entity_type_id);
CREATE INDEX idx_eav_records_entity_type_updated_at
    ON eav_records(entity_type_id, updated_at);
CREATE INDEX idx_eav_records_deleted_at ON eav_records(deleted_at);
CREATE INDEX idx_eav_records_status ON eav_records(status) WHERE deleted_at IS NULL;

-- Optimistic locking: any update of a live record must bump rev by exactly 1.
-- Soft deletes (setting deleted_at) are exempt, mirroring the SQLite trigger.
CREATE FUNCTION devengine_eav_records_check_rev() RETURNS trigger AS $$
BEGIN
    IF NEW.rev != OLD.rev + 1 THEN
        RAISE EXCEPTION 'eav_records: rev must be incremented by exactly 1';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_eav_records_update_rev
    BEFORE UPDATE ON eav_records
    FOR EACH ROW
    WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NULL)
    EXECUTE FUNCTION devengine_eav_records_check_rev();

CREATE TABLE eav_values (
    record_id     BIGINT NOT NULL
        REFERENCES eav_records(id) ON DELETE CASCADE,

    attribute_id  BIGINT NOT NULL
        REFERENCES eav_attributes(id) ON DELETE CASCADE,

    v_bool        BOOLEAN,
    v_int         BIGINT,
    v_real        DOUBLE PRECISION,
    v_text        TEXT,
    v_datetime    TEXT, -- ISO-8601 UTC text

    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (record_id, attribute_id),

    -- Enforce exactly one value column set.
    CHECK (num_nonnulls(v_bool, v_int, v_real, v_text, v_datetime) = 1)
);

CREATE INDEX idx_eav_values_attribute_id ON eav_values(attribute_id);
CREATE INDEX idx_eav_values_attr_v_int ON eav_values(attribute_id, v_int);
CREATE INDEX idx_eav_values_attr_v_real ON eav_values(attribute_id, v_real);
CREATE INDEX idx_eav_values_attr_v_datetime ON eav_values(attribute_id, v_datetime);
CREATE INDEX idx_eav_values_attr_v_text_nocase ON eav_values(attribute_id, LOWER(v_text));
