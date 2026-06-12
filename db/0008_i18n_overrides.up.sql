-- 0008_i18n_overrides.up.sql
-- User adjustments to UI translations, layered over the built-in
-- dictionaries at boot. The key IS the US English source string.

CREATE TABLE IF NOT EXISTS i18n_overrides (
    id          INTEGER PRIMARY KEY,
    locale      TEXT NOT NULL,
    msg_key     TEXT NOT NULL,
    translation TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (locale, msg_key)
);
