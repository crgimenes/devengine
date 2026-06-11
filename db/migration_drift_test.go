package db

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/crgimenes/devengine/log"
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

// A brand-new migration appended to the set must NOT read as drift, while an
// in-place edit of an applied migration must.
func TestAppendedMigrationIsNotDrift(t *testing.T) {
	s, err := NewWithPath(filepath.Join(t.TempDir(), "append.db"))
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	defer s.Close()

	fsys := fstest.MapFS{
		"0001_a.up.sql": {Data: []byte("CREATE TABLE a (id INTEGER);")},
		"0002_b.up.sql": {Data: []byte("CREATE TABLE b (id INTEGER);")},
	}
	entries, err := collectMigrations(fsys)
	if err != nil {
		t.Fatalf("collectMigrations: %v", err)
	}
	older, all := entries[:1], entries

	var logBuf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(prev)

	// Database that applied only 0001, now seeing 0001+0002: no drift.
	err = s.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, migrationsFingerprint(older)))
	if err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	checkSchemaDrift(s, older, all)
	if strings.Contains(logBuf.String(), "WARNING") {
		t.Fatalf("appended migration flagged as drift: %s", logBuf.String())
	}
	var got int64
	err = s.QueryRow(`PRAGMA user_version`).Scan(&got)
	if err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if got != migrationsFingerprint(all) {
		t.Fatalf("fingerprint = %d, want full set", got)
	}

	// Database whose applied files were edited afterwards: drift.
	logBuf.Reset()
	err = s.Exec(`PRAGMA user_version = 12345`)
	if err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	checkSchemaDrift(s, older, all)
	if !strings.Contains(logBuf.String(), "WARNING") {
		t.Fatal("edited migration not flagged as drift")
	}
}
