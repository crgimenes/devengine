package postgres

import (
	"embed"
	"fmt"
	"hash/fnv"
	"io/fs"
	"sort"
	"strings"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/log"
)

var (
	//go:embed *.up.sql
	engineMigrationsFS embed.FS

	// appMigrationsFS holds the optional application migrations filesystem.
	// Set via SetAppMigrationsFS before calling RunMigration.
	appMigrationsFS fs.FS
)

// SetAppMigrationsFS configures an optional fs.FS containing application
// migrations written in PostgreSQL dialect. Files follow the same pattern
// as the SQLite backend: NNNN_name.up.sql, engine 0001-0999, app 1000-9999.
func SetAppMigrationsFS(fsys fs.FS) {
	appMigrationsFS = fsys
}

// migrationEntry represents a single migration file to be applied.
type migrationEntry struct {
	id       string // e.g., "0001_users_and_files"
	filename string // e.g., "0001_users_and_files.up.sql"
	fsys     fs.FS  // the filesystem containing this file
}

// parseMigrationFilename extracts the migration ID from a filename.
// Expected format: NNNN_name.up.sql where NNNN is a 4-digit number.
func parseMigrationFilename(filename string) (string, error) {
	if !strings.HasSuffix(filename, ".up.sql") {
		return "", fmt.Errorf("invalid migration filename %q: must end with .up.sql", filename)
	}
	id := strings.TrimSuffix(filename, ".up.sql")
	if len(id) < 6 { // minimum: "0001_x"
		return "", fmt.Errorf("invalid migration filename %q: too short", filename)
	}
	prefix := id[:4]
	for _, c := range prefix {
		if c < '0' || c > '9' {
			return "", fmt.Errorf("invalid migration filename %q: must start with 4-digit prefix", filename)
		}
	}
	if id[4] != '_' {
		return "", fmt.Errorf("invalid migration filename %q: digit prefix must be followed by underscore", filename)
	}
	return id, nil
}

// collectMigrations reads all .up.sql files from a filesystem.
func collectMigrations(fsys fs.FS) ([]migrationEntry, error) {
	if fsys == nil {
		return nil, nil
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}
	var migrations []migrationEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		id, err := parseMigrationFilename(name)
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, migrationEntry{
			id:       id,
			filename: name,
			fsys:     fsys,
		})
	}
	return migrations, nil
}

func chkTableExists(tx db.Tx) (bool, error) {
	const query = `SELECT count(*)
                       FROM information_schema.tables
                       WHERE table_schema = current_schema()
                       AND table_name = 'schema_migrations'`
	var count int
	err := tx.QueryRow(query).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check if schema_migrations table exists: %w", err)
	}
	return count > 0, nil
}

func createMigrationsTable(tx db.Tx) error {
	const createTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
		id TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT now())`
	err := tx.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}
	return nil
}

// getAppliedMigrations returns a set of migration IDs already applied.
func getAppliedMigrations(tx db.Tx) (map[string]bool, error) {
	const query = "SELECT id FROM schema_migrations"
	rows, err := tx.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration id: %w", err)
		}
		applied[id] = true
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("error iterating applied migrations: %w", err)
	}
	return applied, nil
}

// recordMigration inserts a migration ID into the schema_migrations table.
func recordMigration(tx db.Tx, id string) error {
	const query = "INSERT INTO schema_migrations (id) VALUES ($1)"
	err := tx.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to record migration %q: %w", id, err)
	}
	return nil
}

// RunMigration applies all pending migrations using the global db.Storage.
func RunMigration() error {
	return RunMigrationOn(db.Storage)
}

// RunMigrationOn applies all pending migrations on the provided store.
func RunMigrationOn(s db.Store) error {
	engineMigrations, err := collectMigrations(engineMigrationsFS)
	if err != nil {
		return fmt.Errorf("failed to collect engine migrations: %w", err)
	}
	appMigrations, err := collectMigrations(appMigrationsFS)
	if err != nil {
		return fmt.Errorf("failed to collect application migrations: %w", err)
	}

	allMigrations := append(engineMigrations, appMigrations...)

	seen := make(map[string]string) // id -> filename
	for _, m := range allMigrations {
		if existing, ok := seen[m.id]; ok {
			return fmt.Errorf("duplicate migration id %q: found in %q and %q", m.id, existing, m.filename)
		}
		seen[m.id] = m.filename
	}

	sort.Slice(allMigrations, func(i, j int) bool {
		return allMigrations[i].id < allMigrations[j].id
	})

	tx, err := s.BeginTransaction()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			rberr := tx.Rollback()
			if rberr != nil {
				log.Printf("failed to rollback transaction: %v", rberr)
			}
		}
	}()

	exists, err := chkTableExists(tx)
	if err != nil {
		return fmt.Errorf("failed to check if schema_migrations table exists: %w", err)
	}
	if !exists {
		err = createMigrationsTable(tx)
		if err != nil {
			return fmt.Errorf("failed to ensure schema_migrations table exists: %w", err)
		}
	}

	applied, err := getAppliedMigrations(tx)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Snapshot of what THIS database had already applied, for the drift
	// check after commit: a brand-new migration must not look like drift.
	var appliedBefore []migrationEntry
	for _, m := range allMigrations {
		if applied[m.id] {
			appliedBefore = append(appliedBefore, m)
		}
	}

	appliedCount := 0
	for _, m := range allMigrations {
		if applied[m.id] {
			continue
		}
		content, err := fs.ReadFile(m.fsys, m.filename)
		if err != nil {
			return fmt.Errorf("failed to read migration file %q: %w", m.filename, err)
		}
		log.Printf("applying migration: %s", m.id)
		err = tx.Exec(string(content))
		if err != nil {
			return fmt.Errorf("failed to apply migration %q: %w", m.id, err)
		}
		err = recordMigration(tx, m.id)
		if err != nil {
			return err
		}
		appliedCount++
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	tx = nil

	checkSchemaDrift(s, appliedBefore, allMigrations)

	if appliedCount == 0 {
		log.Printf("no new migrations to apply")
		return nil
	}
	log.Printf("applied %d migration(s)", appliedCount)
	return nil
}

// checkSchemaDrift warns when the embedded migration files changed after
// this database applied them (in-place edits during early development).
// PostgreSQL has no PRAGMA user_version, so the fingerprint lives in a
// devengine_meta key/value table.
func checkSchemaDrift(s db.Store, appliedBefore, all []migrationEntry) {
	expected := migrationsFingerprint(appliedBefore)
	full := migrationsFingerprint(all)

	err := s.Exec(`CREATE TABLE IF NOT EXISTS devengine_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL)`)
	if err != nil {
		log.Printf("schema fingerprint: %v", err)
		return
	}

	var stored int64
	err = s.QueryRow(`SELECT COALESCE(
		(SELECT value::bigint FROM devengine_meta WHERE key = 'migrations_fingerprint'),
		0)`).Scan(&stored)
	if err != nil {
		return
	}
	if stored != 0 && stored != full && stored != expected {
		log.Printf("WARNING: migration files changed after this database applied them (in-place edits during early development); the schema may be outdated. Recreate the database to pick up the changes.")
	}
	if stored == full {
		return
	}
	err = s.Exec(`INSERT INTO devengine_meta (key, value) VALUES ('migrations_fingerprint', $1)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		fmt.Sprintf("%d", full))
	if err != nil {
		log.Printf("schema fingerprint: %v", err)
	}
}

// migrationsFingerprint hashes id+content of the given migrations.
func migrationsFingerprint(migrations []migrationEntry) int64 {
	h := fnv.New32a()
	for _, m := range migrations {
		content, err := fs.ReadFile(m.fsys, m.filename)
		if err != nil {
			continue
		}
		_, _ = h.Write([]byte(m.id))
		_, _ = h.Write(content)
	}
	return int64(h.Sum32() & 0x7fffffff)
}
