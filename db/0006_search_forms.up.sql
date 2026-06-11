-- 0006_search_forms.up.sql
-- Search forms: a form flagged as search opens on its record listing (which
-- carries the text search box) instead of the create view. Several search
-- forms may point at the same entity type, each exposing a different set of
-- columns through its elements.

ALTER TABLE forms ADD COLUMN is_search INTEGER NOT NULL DEFAULT 0 CHECK (is_search IN (0, 1));
