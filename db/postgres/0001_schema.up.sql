-- 0001_schema.up.sql
-- Complete devengine schema (PostgreSQL dialect), unified pre-1.0.
-- While the engine is experimental there is no migration history: schema
-- changes edit this file in place and databases are recreated from scratch.
-- Real append-only migrations start at 1.0.0. Application migrations keep
-- using ids 1000-9999 on top of this file.

-- Core user accounts, magic links, and file storage (PostgreSQL dialect).
-- Differences from the SQLite migration: IDENTITY ids, BOOLEAN flags,
-- TIMESTAMPTZ timestamps, plpgsql trigger functions, and a generated
-- tsvector column replacing the FTS5 virtual table.

-- Shared trigger function: refresh updated_at unless the statement set it.
CREATE FUNCTION devengine_set_updated_at() RETURNS trigger AS $$
BEGIN
    IF NEW.updated_at IS NOT DISTINCT FROM OLD.updated_at THEN
        NEW.updated_at := now();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Shared trigger function: fill an empty reference_id with an opaque
-- 48-char hex id (same shape as the SQLite randomblob trigger).
CREATE FUNCTION devengine_set_reference_id() RETURNS trigger AS $$
BEGIN
    IF NEW.reference_id IS NULL OR NEW.reference_id = '' THEN
        NEW.reference_id := substr(
            replace(gen_random_uuid()::text || gen_random_uuid()::text, '-', ''),
            1, 48);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Users: reference_id keeps the UUID v4 shape of the SQLite trigger.
CREATE FUNCTION devengine_set_user_reference_id() RETURNS trigger AS $$
BEGIN
    IF NEW.reference_id IS NULL OR NEW.reference_id = '' THEN
        NEW.reference_id := gen_random_uuid()::text;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reference_id TEXT NOT NULL DEFAULT '',
    username TEXT UNIQUE,
    email TEXT UNIQUE,
    password_hash TEXT, -- nullable; populated by basic auth
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    sysop BOOLEAN NOT NULL DEFAULT FALSE,
    avatar_url TEXT,
    -- Per-user UI language. Empty means "no preference": the request
    -- falls back to Accept-Language and then to the app default locale.
    locale TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_reference_id ON users(reference_id) WHERE reference_id != '';
CREATE UNIQUE INDEX idx_users_username_nocase
    ON users(LOWER(username)) WHERE username IS NOT NULL;
CREATE UNIQUE INDEX idx_users_email_nocase
    ON users(LOWER(email)) WHERE email IS NOT NULL;
CREATE INDEX idx_users_enabled ON users(enabled);

CREATE TRIGGER users_reference_uuid
    BEFORE INSERT ON users
    FOR EACH ROW EXECUTE FUNCTION devengine_set_user_reference_id();
CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION devengine_set_updated_at();

CREATE TABLE magic_token (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE,
    action TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '3 hours')
);

CREATE TABLE filemanager_files (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_filename TEXT NOT NULL,
    filename TEXT NOT NULL UNIQUE,
    filesize BIGINT NOT NULL,
    filetype TEXT,
    filehash TEXT,
    filetag TEXT,
    filedescription TEXT,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    -- Replaces the SQLite FTS5 virtual table: one generated tsvector kept
    -- in sync by the engine itself, searched with plainto_tsquery.
    fts tsvector GENERATED ALWAYS AS (
        to_tsvector('simple',
            coalesce(original_filename, '') || ' ' ||
            coalesce(filename, '') || ' ' ||
            coalesce(filetag, '') || ' ' ||
            coalesce(filedescription, ''))
    ) STORED
);

CREATE INDEX idx_filemanager_files_user_id ON filemanager_files(user_id);
CREATE INDEX idx_filemanager_files_user_id_deleted
    ON filemanager_files(user_id, deleted);
CREATE INDEX idx_filemanager_files_hash ON filemanager_files(filehash);
CREATE INDEX idx_filemanager_files_fts ON filemanager_files USING GIN (fts);

CREATE TRIGGER filemanager_files_set_updated_at
    BEFORE UPDATE ON filemanager_files
    FOR EACH ROW EXECUTE FUNCTION devengine_set_updated_at();

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

-- Menus / Navigation layer (PostgreSQL dialect).

CREATE TABLE menus (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reference_id  TEXT NOT NULL DEFAULT '',

    machine_name  TEXT NOT NULL,
    label         TEXT NOT NULL,
    description   TEXT,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_menus_reference_id_active
    ON menus(reference_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_menus_machine_name_active
    ON menus(LOWER(machine_name)) WHERE deleted_at IS NULL;
CREATE INDEX idx_menus_deleted_at ON menus(deleted_at);

CREATE TRIGGER trg_menus_reference_id
    BEFORE INSERT ON menus
    FOR EACH ROW EXECUTE FUNCTION devengine_set_reference_id();
CREATE TRIGGER menus_set_updated_at
    BEFORE UPDATE ON menus
    FOR EACH ROW EXECUTE FUNCTION devengine_set_updated_at();

CREATE TABLE menu_items (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reference_id  TEXT NOT NULL DEFAULT '',

    menu_id       BIGINT NOT NULL
        REFERENCES menus(id) ON DELETE CASCADE,

    parent_id     BIGINT
        REFERENCES menu_items(id) ON DELETE CASCADE,

    machine_name  TEXT NOT NULL,
    label         TEXT NOT NULL,
    icon          TEXT,

    item_type     TEXT NOT NULL DEFAULT 'link' CHECK (item_type IN ('link', 'separator', 'submenu')),

    url           TEXT,
    js_code       TEXT,
    filo_code     TEXT,

    z_order       INTEGER NOT NULL DEFAULT 0,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,

    UNIQUE(menu_id, machine_name)
);

CREATE UNIQUE INDEX idx_menu_items_reference_id_active
    ON menu_items(reference_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_menu_items_menu_order
    ON menu_items(menu_id, parent_id, z_order, id);
CREATE INDEX idx_menu_items_parent_order
    ON menu_items(parent_id, z_order, id);
CREATE INDEX idx_menu_items_deleted_at ON menu_items(deleted_at);

CREATE TRIGGER trg_menu_items_reference_id
    BEFORE INSERT ON menu_items
    FOR EACH ROW EXECUTE FUNCTION devengine_set_reference_id();
CREATE TRIGGER menu_items_set_updated_at
    BEFORE UPDATE ON menu_items
    FOR EACH ROW EXECUTE FUNCTION devengine_set_updated_at();

-- Forms / UI projection layer (PostgreSQL dialect).

CREATE TABLE forms (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reference_id  TEXT NOT NULL DEFAULT '',

    machine_name  TEXT NOT NULL,
    label         TEXT NOT NULL,
    description   TEXT,

    eav_entity_type_id BIGINT
        REFERENCES eav_entity_types(id) ON DELETE SET NULL,

    hide_submit_button BOOLEAN NOT NULL DEFAULT FALSE,
    hide_cancel_button BOOLEAN NOT NULL DEFAULT FALSE,
    hide_title         BOOLEAN NOT NULL DEFAULT FALSE,
    show_system_info   BOOLEAN NOT NULL DEFAULT FALSE,

    -- Associated menu for navbar display when form is active
    menu_id BIGINT REFERENCES menus(id) ON DELETE SET NULL,

    -- Search form: /form/{name} opens the record listing (search view)
    -- instead of the create view. Several search forms may point at the
    -- same entity type, each exposing different columns.
    is_search BOOLEAN NOT NULL DEFAULT FALSE,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_forms_reference_id_active
    ON forms(reference_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_forms_machine_name_active
    ON forms(LOWER(machine_name)) WHERE deleted_at IS NULL;
CREATE INDEX idx_forms_deleted_at ON forms(deleted_at);

CREATE TRIGGER trg_forms_reference_id
    BEFORE INSERT ON forms
    FOR EACH ROW EXECUTE FUNCTION devengine_set_reference_id();
CREATE TRIGGER forms_set_updated_at
    BEFORE UPDATE ON forms
    FOR EACH ROW EXECUTE FUNCTION devengine_set_updated_at();

CREATE TABLE form_elements (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reference_id  TEXT NOT NULL DEFAULT '',

    form_id       BIGINT NOT NULL
        REFERENCES forms(id) ON DELETE CASCADE,

    parent_id     BIGINT
        REFERENCES form_elements(id) ON DELETE CASCADE,

    machine_name  TEXT NOT NULL,
    element_kind  TEXT NOT NULL,

    label         TEXT,
    help_text     TEXT,

    z_order       INTEGER NOT NULL DEFAULT 0,
    col_span      INTEGER NOT NULL DEFAULT 12 CHECK (col_span >= 1 AND col_span <= 12),
    alignment     TEXT NOT NULL DEFAULT 'left' CHECK (alignment IN ('left', 'center', 'right')),

    ui_kind       TEXT,
    ui_meta_json  TEXT,

    eav_attribute_id BIGINT
        REFERENCES eav_attributes(id) ON DELETE RESTRICT,

    is_ui_only    BOOLEAN NOT NULL DEFAULT FALSE,
    is_readonly   BOOLEAN NOT NULL DEFAULT FALSE,
    hide_label    BOOLEAN NOT NULL DEFAULT FALSE,
    hide_help_text BOOLEAN NOT NULL DEFAULT FALSE,

    validate_expr  TEXT,
    computed_expr  TEXT,

    button_filo_code   TEXT,
    button_run_save    BOOLEAN NOT NULL DEFAULT FALSE,
    button_js_code     TEXT,
    button_style       TEXT DEFAULT 'primary',
    button_confirm_msg TEXT,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,

    UNIQUE(form_id, machine_name)
);

CREATE UNIQUE INDEX idx_form_elements_reference_id_active
    ON form_elements(reference_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_form_elements_form_order
    ON form_elements(form_id, parent_id, z_order, id);
CREATE INDEX idx_form_elements_parent_order
    ON form_elements(parent_id, z_order, id);
CREATE INDEX idx_form_elements_deleted_at ON form_elements(deleted_at);

CREATE TRIGGER trg_form_elements_reference_id
    BEFORE INSERT ON form_elements
    FOR EACH ROW EXECUTE FUNCTION devengine_set_reference_id();
CREATE TRIGGER form_elements_set_updated_at
    BEFORE UPDATE ON form_elements
    FOR EACH ROW EXECUTE FUNCTION devengine_set_updated_at();

-- v_eav_cells
-- Generic EAV view exposing cells as a relational stream (PostgreSQL).

CREATE OR REPLACE VIEW v_eav_cells AS
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

-- User adjustments to UI translations, layered over the built-in
-- dictionaries at boot. The key IS the US English source string.

CREATE TABLE i18n_overrides (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    locale      TEXT NOT NULL,
    msg_key     TEXT NOT NULL,
    translation TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (locale, msg_key)
);

-- Translations for user-created content (form labels, element labels and
-- help text, attribute labels, menu item labels). ref_id is the object's
-- reference_id; field names the translated column.

CREATE TABLE content_translations (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    locale     TEXT NOT NULL,
    ref_id     TEXT NOT NULL,
    field      TEXT NOT NULL,
    text       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (locale, ref_id, field)
);
