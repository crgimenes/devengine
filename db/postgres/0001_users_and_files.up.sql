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
