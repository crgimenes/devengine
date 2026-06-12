package sqlite

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
)

func TestParseMigrationFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantID   string
		wantErr  string
	}{
		{
			name:     "valid engine migration",
			filename: "0001_users.up.sql",
			wantID:   "0001_users",
		},
		{
			name:     "valid app migration",
			filename: "1000_characters.up.sql",
			wantID:   "1000_characters",
		},
		{
			name:     "valid with underscores in name",
			filename: "0003_session_data.up.sql",
			wantID:   "0003_session_data",
		},
		{
			name:     "missing .up.sql suffix",
			filename: "0001_users.sql",
			wantErr:  "must end with .up.sql",
		},
		{
			name:     "too short",
			filename: "01_x.up.sql",
			wantErr:  "too short",
		},
		{
			name:     "3-digit prefix (old format)",
			filename: "001_users.up.sql",
			wantErr:  "must start with 4-digit prefix",
		},
		{
			name:     "no underscore after digits",
			filename: "0001users.up.sql",
			wantErr:  "digit prefix must be followed by underscore",
		},
		{
			name:     "non-numeric prefix",
			filename: "abcd_users.up.sql",
			wantErr:  "must start with 4-digit prefix",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := parseMigrationFilename(tc.filename)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tc.wantID {
				t.Fatalf("expected ID %q, got %q", tc.wantID, id)
			}
		})
	}
}

func TestCollectMigrations(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_users.up.sql":       &fstest.MapFile{Data: []byte("CREATE TABLE users")},
		"0002_permissions.up.sql": &fstest.MapFile{Data: []byte("CREATE TABLE perms")},
		"readme.txt":              &fstest.MapFile{Data: []byte("not a migration")},
	}

	migrations, err := collectMigrations(fsys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}

	// Verify IDs (order may vary before sorting)
	ids := make(map[string]bool)
	for _, m := range migrations {
		ids[m.id] = true
	}

	if !ids["0001_users"] {
		t.Error("expected migration 0001_users not found")
	}
	if !ids["0002_permissions"] {
		t.Error("expected migration 0002_permissions not found")
	}
}

func TestCollectMigrationsInvalidFile(t *testing.T) {
	fsys := fstest.MapFS{
		"invalid.up.sql": &fstest.MapFile{Data: []byte("bad")},
	}

	_, err := collectMigrations(fsys)
	if err == nil {
		t.Fatal("expected error for invalid filename, got nil")
	}
	if !strings.Contains(err.Error(), "must start with 4-digit prefix") {
		t.Fatalf("expected error about 4-digit prefix, got %q", err)
	}
}

func TestFindMigrationFile(t *testing.T) {
	tests := []struct {
		name    string
		fsys    fstest.MapFS
		version int
		want    string
		wantErr string
	}{
		{
			name: "match 4-digit prefix",
			fsys: fstest.MapFS{
				"0001_initial.up.sql": &fstest.MapFile{Mode: 0o644, Data: []byte("-- migration")},
			},
			version: 1,
			want:    "0001_initial.up.sql",
		},
		{
			name: "match version 1000",
			fsys: fstest.MapFS{
				"1000_characters.up.sql": &fstest.MapFile{Mode: 0o644, Data: []byte("-- migration")},
			},
			version: 1000,
			want:    "1000_characters.up.sql",
		},
		{
			name: "missing version",
			fsys: fstest.MapFS{
				"0004_extra.up.sql": &fstest.MapFile{Mode: 0o644, Data: []byte("-- migration")},
			},
			version: 1,
			wantErr: "no migration file",
		},
		{
			name: "multiple matches with same version",
			fsys: fstest.MapFS{
				"0005_a.up.sql": &fstest.MapFile{Mode: 0o644, Data: []byte("CREATE TABLE a")},
				"0005_b.up.sql": &fstest.MapFile{Mode: 0o644, Data: []byte("CREATE TABLE b")},
			},
			version: 5,
			wantErr: "multiple migration files",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := findMigrationFile(tc.fsys, tc.version)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q but got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q but got %q", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestRunMigrations(t *testing.T) {
	// Use temp file for database
	tmp := t.TempDir()
	config.Cfg.DBFile = filepath.Join(tmp, "test.db")

	var err error
	db.Storage, err = New()
	if err != nil {
		t.Fatalf("Error on db: %s", err)
	}
	defer db.Storage.Close()

	// First run
	err = RunMigration()
	if err != nil {
		t.Fatalf("Migration error: %v", err)
	}

	// Run again to test idempotency
	err = RunMigration()
	if err != nil {
		t.Fatalf("Second migration error: %v", err)
	}
}

func TestRunMigrationsWithAppFS(t *testing.T) {
	// Use temp file for database (not :memory:) so RW and RO pools share same DB
	tmp := t.TempDir()
	config.Cfg.DBFile = filepath.Join(tmp, "test.db")

	var err error
	db.Storage, err = New()
	if err != nil {
		t.Fatalf("Error on db: %s", err)
	}
	defer db.Storage.Close()

	// Set up app migrations
	appFS := fstest.MapFS{
		"1000_test_app_table.up.sql": &fstest.MapFile{
			Data: []byte("CREATE TABLE test_app (id INTEGER PRIMARY KEY)"),
		},
		"1001_test_app_data.up.sql": &fstest.MapFile{
			Data: []byte("INSERT INTO test_app (id) VALUES (1)"),
		},
	}
	SetAppMigrationsFS(appFS)
	defer SetAppMigrationsFS(nil) // reset after test

	err = RunMigration()
	if err != nil {
		t.Fatalf("Migration error: %v", err)
	}

	// Verify app table exists
	var count int
	err = db.Storage.QueryRow("SELECT COUNT(*) FROM test_app").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query test_app: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 row in test_app, got %d", count)
	}

	// Run again to test idempotency
	err = RunMigration()
	if err != nil {
		t.Fatalf("Second migration error: %v", err)
	}
}

func TestDuplicateMigrationID(t *testing.T) {
	tmp := t.TempDir()
	config.Cfg.DBFile = filepath.Join(tmp, "test.db")

	var err error
	db.Storage, err = New()
	if err != nil {
		t.Fatalf("Error on db: %s", err)
	}
	defer db.Storage.Close()

	// Create app migrations that duplicate an engine migration ID
	// Use exact same ID as engine migration to trigger duplicate detection
	appFS := fstest.MapFS{
		"0001_schema.up.sql": &fstest.MapFile{
			Data: []byte("-- duplicate of engine migration"),
		},
	}
	SetAppMigrationsFS(appFS)
	defer SetAppMigrationsFS(nil)

	err = RunMigration()
	if err == nil {
		t.Fatal("Expected error for duplicate migration ID, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate migration id") {
		t.Fatalf("Expected duplicate ID error, got: %v", err)
	}
}
