-- Core user accounts, identities, magic links, and file storage (with FTS)

CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    reference_id TEXT NOT NULL DEFAULT '' UNIQUE, -- a trigger will set this to a UUID
    username TEXT UNIQUE COLLATE NOCASE,
    email TEXT UNIQUE COLLATE NOCASE,
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0,1)),
    sysop INTEGER NOT NULL DEFAULT 0 CHECK (sysop IN (0,1)),
    avatar_url TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_username_nocase
    ON users(LOWER(username)) WHERE username IS NOT NULL;
CREATE INDEX idx_users_email_nocase
    ON users(LOWER(email)) WHERE email IS NOT NULL;
CREATE INDEX idx_users_enabled ON users(enabled);
CREATE INDEX idx_users_reference_id ON users(reference_id);

CREATE TABLE identities (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_uid TEXT NOT NULL,
    avatar_url TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, provider_uid)
);

CREATE INDEX idx_identities_user_id ON identities(user_id);

CREATE TRIGGER users_set_updated_at
AFTER UPDATE ON users
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
        UPDATE users
        SET updated_at = CURRENT_TIMESTAMP
        WHERE id = NEW.id;
END;

CREATE TRIGGER identities_set_updated_at
AFTER UPDATE ON identities
FOR EACH ROW
WHEN NEW.updated_at = OLD.updated_at
BEGIN
        UPDATE identities
        SET updated_at = CURRENT_TIMESTAMP
        WHERE id = NEW.id;
END;

CREATE TRIGGER users_reference_uuid
AFTER INSERT ON users
BEGIN
  UPDATE users
  SET reference_id = (
    select substr(u,1,8)||'-'||
    substr(u,9,4)||'-4'||
    substr(u,13,3)||'-'||v||
    substr(u,17,3)||'-'||
    substr(u,21,12) from (
        select
            lower(hex(randomblob(16))) as u,
            substr('89ab',abs(random()) % 4 + 1, 1) as v)
    )
  WHERE id = NEW.id;
END;

CREATE TABLE magic_token (
    id INTEGER PRIMARY KEY,
    email TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE,
    action TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL DEFAULT (DATETIME('now', '+3 hour'))
);

CREATE TABLE filemanager_files (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- owner of the file
    original_filename TEXT NOT NULL,
    filename TEXT NOT NULL UNIQUE,
    filesize INTEGER NOT NULL, -- in bytes
    filetype TEXT,
    filehash TEXT, -- e.g. SHA256 hash of the file
    filetag TEXT, -- e.g. category or tag for the file (user_avatar, document, etc.)
    filedescription TEXT,
    processed INTEGER NOT NULL DEFAULT 0 CHECK (processed IN (0,1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted INTEGER NOT NULL DEFAULT 0 CHECK (deleted IN (0,1))
);

CREATE INDEX idx_filemanager_files_user_id ON filemanager_files(user_id);
CREATE INDEX idx_filemanager_files_user_id_deleted
    ON filemanager_files(user_id, deleted);
CREATE INDEX idx_filemanager_files_hash ON filemanager_files(filehash);

CREATE TRIGGER filemanager_files_set_updated_at
AFTER UPDATE OF original_filename, filename, filesize, filetype, filehash, filetag, filedescription, processed ON filemanager_files
BEGIN
    UPDATE filemanager_files SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

CREATE TRIGGER filemanager_files_set_updated_at_on_deleted
AFTER UPDATE ON filemanager_files
FOR EACH ROW
WHEN NEW.deleted != OLD.deleted
BEGIN
    UPDATE filemanager_files SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

CREATE VIRTUAL TABLE filemanager_files_fts
USING fts5(
    original_filename,
    filename,
    filetag,
    filedescription,
    content='filemanager_files',
    content_rowid='id'
);

CREATE TRIGGER filemanager_files_ai
AFTER INSERT ON filemanager_files
BEGIN
  INSERT INTO filemanager_files_fts(rowid, original_filename, filename, filetag, filedescription)
    SELECT NEW.id, NEW.original_filename, NEW.filename, NEW.filetag, NEW.filedescription WHERE NEW.deleted = 0;
END;

CREATE TRIGGER filemanager_files_ad
AFTER DELETE ON filemanager_files
BEGIN
  INSERT INTO filemanager_files_fts(filemanager_files_fts, rowid)
    VALUES('delete', OLD.id);
END;

CREATE TRIGGER filemanager_files_au
AFTER UPDATE OF original_filename, filename, filetag, filedescription, deleted ON filemanager_files
BEGIN
  -- remove old index entry
  INSERT INTO filemanager_files_fts(filemanager_files_fts, rowid)
    VALUES('delete', OLD.id);
  -- add new entry only if not deleted
  INSERT INTO filemanager_files_fts(rowid, original_filename, filename, filetag, filedescription)
    SELECT NEW.id, NEW.original_filename, NEW.filename, NEW.filetag, NEW.filedescription WHERE NEW.deleted = 0;
END;
