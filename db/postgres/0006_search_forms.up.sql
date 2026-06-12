-- Search forms: a form flagged as search opens on its record listing (which
-- carries the text search box) instead of the create view.

ALTER TABLE forms ADD COLUMN is_search BOOLEAN NOT NULL DEFAULT FALSE;
