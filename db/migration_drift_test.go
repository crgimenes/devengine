package db

import (
	"path/filepath"
	"testing"
)

// The schema fingerprint must be stamped on first migration and restored
// after drift is detected, so the warning fires once per change.
func TestSchemaDriftFingerprint(t *testing.T) {
	s, err := NewWithPath(filepath.Join(t.TempDir(), "drift.db"))
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	defer s.Close()

	err = RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}

	var fingerprint int64
	err = s.QueryRow(`PRAGMA user_version`).Scan(&fingerprint)
	if err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if fingerprint == 0 {
		t.Fatal("fingerprint not stamped after migration")
	}

	// Simulate a database created before the migrations were edited.
	err = s.Exec(`PRAGMA user_version = 1`)
	if err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	err = RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn (drift): %v", err)
	}

	var restored int64
	err = s.QueryRow(`PRAGMA user_version`).Scan(&restored)
	if err != nil {
		t.Fatalf("re-read user_version: %v", err)
	}
	if restored != fingerprint {
		t.Fatalf("fingerprint = %d, want %d", restored, fingerprint)
	}
}
