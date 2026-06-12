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
