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

-- forms.menu_id FK deferred from 0003 (menus did not exist yet there).
ALTER TABLE forms ADD CONSTRAINT fk_forms_menu
    FOREIGN KEY (menu_id) REFERENCES menus(id) ON DELETE SET NULL;

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
