-- Per-user UI language. Empty means "no preference": the request falls back
-- to Accept-Language and then to the application default locale.

ALTER TABLE users ADD COLUMN locale TEXT NOT NULL DEFAULT '';
