package sqlite

import (
	"path/filepath"
	"testing"
	"testing/fstest"
)

// Application migrations (ids 1000+) apply on top of the unified engine
// schema and can reference engine tables.
func TestAppMigrationsApply(t *testing.T) {
	s, err := NewWithPath(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	t.Cleanup(s.Close)

	appFS := fstest.MapFS{
		"1000_posts.up.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE app_posts (
				id INTEGER PRIMARY KEY,
				author_id INTEGER NOT NULL REFERENCES users(id),
				title TEXT NOT NULL
			);`)},
		"1001_posts_index.up.sql": &fstest.MapFile{Data: []byte(`
			CREATE INDEX idx_app_posts_author ON app_posts(author_id);`)},
	}
	SetAppMigrationsFS(appFS)
	t.Cleanup(func() { SetAppMigrationsFS(nil) })

	err = RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}

	// Engine table and app table coexist; the FK to users resolves.
	u, err := s.CreateUser("autor", "autor@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	err = s.Exec("INSERT INTO app_posts (author_id, title) VALUES (?, ?)", u.ID, "hello")
	if err != nil {
		t.Fatalf("insert into app table: %v", err)
	}

	// Idempotent: a second run applies nothing and does not error.
	err = RunMigrationOn(s)
	if err != nil {
		t.Fatalf("second RunMigrationOn: %v", err)
	}

	// App tables surface in the schema viewer listing.
	tables, err := s.ListRelationalTables()
	if err != nil {
		t.Fatalf("ListRelationalTables: %v", err)
	}
	seen := false
	for _, tb := range tables {
		if tb.Name == "app_posts" {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("app_posts missing from schema listing: %v", tables)
	}
}
