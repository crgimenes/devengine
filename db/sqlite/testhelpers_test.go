package sqlite

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/crgimenes/devengine/db"
)

// testMigrationMu serializes schema migrations across tests because RunMigration
// mutates the global db.Storage variable.
var testMigrationMu sync.Mutex

// initTestDB opens a temp-file SQLite database and runs all engine migrations
// against it. The returned *SQLite is isolated per test and cleaned up when
// t.TempDir() goes away.
func initTestDB(t *testing.T) *SQLite {
	t.Helper()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.db")

	s, err := NewWithPath(path)
	if err != nil {
		t.Fatalf("NewWithPath(%q): %v", path, err)
	}

	testMigrationMu.Lock()
	oldStorage := db.Storage
	db.Storage = s
	err = RunMigration()
	db.Storage = oldStorage
	testMigrationMu.Unlock()

	if err != nil {
		t.Fatalf("RunMigration: %v", err)
	}

	return s
}
