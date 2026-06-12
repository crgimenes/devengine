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

    -- FK added in 0005 (menus does not exist yet at this point; the SQLite
    -- migration can forward-reference, PostgreSQL cannot).
    menu_id BIGINT,

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
