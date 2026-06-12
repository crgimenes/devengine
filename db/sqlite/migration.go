package sqlite

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
// migrations. Files must follow the pattern: NNNN_name.up.sql where NNNN
// is a 4-digit prefix (e.g., 1000_characters.up.sql).
//
// Migration ordering:
//   - Engine migrations (devengine): 0001-0999
//   - Application migrations: 1000-9999
//
// Call this function before RunMigration() in your application's main.go:
//
//	sqlite.SetAppMigrationsFS(migrations.FS)
//	err := sqlite.RunMigration()
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
// Returns the ID (without .up.sql extension) and any error.
func parseMigrationFilename(filename string) (string, error) {
	if !strings.HasSuffix(filename, ".up.sql") {
		return "", fmt.Errorf("invalid migration filename %q: must end with .up.sql", filename)
	}

	// Remove .up.sql suffix to get the ID
	id := strings.TrimSuffix(filename, ".up.sql")

	// Validate format: must start with 4 digits followed by underscore
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

// collectMigrations reads all .up.sql files from a filesystem and returns migration entries.
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
                       FROM sqlite_master
                       WHERE type='table'
                       AND name='schema_migrations'`
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
		applied_at TEXT DEFAULT CURRENT_TIMESTAMP)`
	err := tx.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}
	return nil
}

// getAppliedMigrations returns a set of migration IDs that have already been applied.
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
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan migration id: %w", err)
		}
		applied[id] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating applied migrations: %w", err)
	}

	return applied, nil
}

// recordMigration inserts a migration ID into the schema_migrations table.
func recordMigration(tx db.Tx, id string) error {
	const query = "INSERT INTO schema_migrations (id) VALUES (?)"
	if err := tx.Exec(query, id); err != nil {
		return fmt.Errorf("failed to record migration %q: %w", id, err)
	}
	return nil
}

// RunMigration applies all pending migrations in order:
//  1. Engine migrations (0001-0999) from devengine/db
//  2. Application migrations (1000+) from the configured AppFS
//
// Migrations are sorted lexicographically by ID, ensuring engine migrations
// run before application migrations due to the numbering convention.
// RunMigration applies all pending migrations using the global db.Storage.
func RunMigration() error {
	return RunMigrationOn(db.Storage)
}

// RunMigrationOn applies all pending migrations on the provided store.
func RunMigrationOn(s db.Store) error {
	// Collect engine migrations
	engineMigrations, err := collectMigrations(engineMigrationsFS)
	if err != nil {
		return fmt.Errorf("failed to collect engine migrations: %w", err)
	}

	// Collect application migrations (if configured)
	appMigrations, err := collectMigrations(appMigrationsFS)
	if err != nil {
		return fmt.Errorf("failed to collect application migrations: %w", err)
	}

	// Merge all migrations
	allMigrations := append(engineMigrations, appMigrations...)

	// Check for duplicate IDs
	seen := make(map[string]string) // id -> filename
	for _, m := range allMigrations {
		if existing, ok := seen[m.id]; ok {
			return fmt.Errorf("duplicate migration id %q: found in %q and %q", m.id, existing, m.filename)
		}
		seen[m.id] = m.filename
	}

	// Sort by ID (lexicographic order ensures 0001 < 0002 < ... < 1000 < 1001)
	sort.Slice(allMigrations, func(i, j int) bool {
		return allMigrations[i].id < allMigrations[j].id
	})

	// Begin transaction
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

	// Ensure schema_migrations table exists
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

	// Get already applied migrations
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

	// Apply pending migrations
	appliedCount := 0
	for _, m := range allMigrations {
		if applied[m.id] {
			continue
		}

		// Read migration file
		content, err := fs.ReadFile(m.fsys, m.filename)
		if err != nil {
			return fmt.Errorf("failed to read migration file %q: %w", m.filename, err)
		}

		// Apply migration
		log.Printf("applying migration: %s", m.id)
		if err := tx.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to apply migration %q: %w", m.id, err)
		}

		// Record migration
		if err := recordMigration(tx, m.id); err != nil {
			return err
		}

		appliedCount++
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
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

// checkSchemaDrift warns when the embedded migration files changed after this
// database applied them. Early development edits migrations in place, so an
// already-migrated database silently keeps the old schema and fails later
// with confusing SQL errors. The fingerprint lives in PRAGMA user_version and
// is refreshed after warning, so the warning fires once per change.
func checkSchemaDrift(s db.Store, appliedBefore, all []migrationEntry) {
	expected := migrationsFingerprint(appliedBefore)
	full := migrationsFingerprint(all)

	var stored int64
	err := s.QueryRow(`PRAGMA user_version`).Scan(&stored)
	if err != nil {
		return
	}
	// Drift means the files this database ALREADY applied changed on disk.
	// A stored value matching neither full (no change at all) nor expected
	// (only new migrations appended) is exactly that.
	if stored != 0 && stored != full && stored != expected {
		log.Printf("WARNING: migration files changed after this database applied them (in-place edits during early development); the schema may be outdated. Recreate the database file to pick up the changes.")
	}
	if stored == full {
		return
	}
	err = s.Exec(fmt.Sprintf("PRAGMA user_version = %d", full))
	if err != nil {
		log.Printf("schema fingerprint: %v", err)
	}
}

// migrationsFingerprint hashes id+content of the given migrations into a
// positive int32 that round-trips through PRAGMA user_version.
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

func findMigrationFile(fsys fs.FS, version int) (string, error) {
	pattern := fmt.Sprintf("%04d_*.up.sql", version)
	matches, err := fs.Glob(fsys, pattern)
	if err != nil {
		return "", fmt.Errorf(
			"failed to glob migration files using pattern %q: %w",
			pattern,
			err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf(
			"no migration file matched pattern %q for version %04d",
			pattern,
			version)
	}

	if len(matches) > 1 {
		// If multiple matches exist, prefer a file that has non-empty, non-comment content.
		// This allows deprecating a migration by leaving an empty/comment-only file with same version.
		nonEmpty := make([]string, 0, len(matches))
		for _, m := range matches {
			b, rerr := fs.ReadFile(fsys, m)
			if rerr != nil {
				// If we cannot read, treat as non-empty to avoid false negatives
				nonEmpty = append(nonEmpty, m)
				continue
			}
			content := strings.TrimSpace(string(b))
			// Strip out leading comment lines
			lines := strings.Split(content, "\n")
			filtered := make([]string, 0, len(lines))
			for _, ln := range lines {
				s := strings.TrimSpace(ln)
				if s == "" {
					continue
				}
				if strings.HasPrefix(s, "--") {
					continue
				}
				filtered = append(filtered, s)
			}
			if len(filtered) > 0 {
				nonEmpty = append(nonEmpty, m)
			}
		}
		if len(nonEmpty) == 1 {
			return nonEmpty[0], nil
		}
		return "", fmt.Errorf(
			"multiple migration files matched pattern %q for version %04d: %v",
			pattern,
			version,
			matches)
	}

	return matches[0], nil
}
