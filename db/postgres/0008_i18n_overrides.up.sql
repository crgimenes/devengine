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
