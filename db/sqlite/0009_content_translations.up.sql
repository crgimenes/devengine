-- 0009_content_translations.up.sql
-- Translations for user-created content (form labels, element labels and
-- help text, attribute labels, menu item labels). ref_id is the object's
-- reference_id; field names the translated column. Applied at render time
-- on runtime screens, falling back to the original text.

CREATE TABLE IF NOT EXISTS content_translations (
    id         INTEGER PRIMARY KEY,
    locale     TEXT NOT NULL,
    ref_id     TEXT NOT NULL,
    field      TEXT NOT NULL,
    text       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (locale, ref_id, field)
);
