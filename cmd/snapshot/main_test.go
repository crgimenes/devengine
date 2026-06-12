package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/crgimenes/devengine/db/sqlite"
)

func TestRunSnapshotsDBAndData(t *testing.T) {
	dir := t.TempDir()

	// Source database with real schema and one user.
	dbPath := filepath.Join(dir, "app.db")
	s, err := sqlite.NewWithPath(dbPath)
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	err = sqlite.RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}
	_, err = s.CreateUser("alice", "alice@example.com", "x", true)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	defer s.Close()

	// Data directory with a nested file.
	dataDir := filepath.Join(dir, "data")
	err = os.MkdirAll(filepath.Join(dataDir, "u1"), 0o750)
	if err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	err = os.WriteFile(filepath.Join(dataDir, "u1", "photo.png"), []byte("png-bytes"), 0o600)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	outDir := filepath.Join(dir, "backups")
	now := time.Date(2026, 6, 11, 13, 0, 0, 0, time.UTC)
	err = run(dbPath, dataDir, outDir, now)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// The snapshot must be a working database containing the user.
	snapPath := filepath.Join(outDir, "20260611-130000.db")
	snap, err := sqlite.NewWithPath(snapPath)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer snap.Close()
	u, err := snap.GetUserByUsername("alice")
	if err != nil || u == nil {
		t.Fatalf("snapshot user lookup: %v %v", u, err)
	}

	// The archive must carry the nested file with its content.
	tarPath := filepath.Join(outDir, "20260611-130000-data.tar.gz")
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	tr := tar.NewReader(gz)
	found := false
	for {
		hdr, rerr := tr.Next()
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			t.Fatalf("tar next: %v", rerr)
		}
		if hdr.Name == "u1/photo.png" {
			content, cerr := io.ReadAll(tr)
			if cerr != nil {
				t.Fatalf("read entry: %v", cerr)
			}
			if string(content) != "png-bytes" {
				t.Fatalf("entry content = %q", content)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("u1/photo.png missing from archive")
	}
}

func TestRunSkipsAbsentDataDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "app.db")
	s, err := sqlite.NewWithPath(dbPath)
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	err = sqlite.RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}
	s.Close()

	outDir := filepath.Join(dir, "backups")
	now := time.Date(2026, 6, 11, 13, 0, 0, 0, time.UTC)
	err = run(dbPath, filepath.Join(dir, "no-such-dir"), outDir, now)
	if err != nil {
		t.Fatalf("run without data dir: %v", err)
	}

	_, err = os.Stat(filepath.Join(outDir, "20260611-130000.db"))
	if err != nil {
		t.Fatalf("db snapshot missing: %v", err)
	}
	_, err = os.Stat(filepath.Join(outDir, "20260611-130000-data.tar.gz"))
	if !os.IsNotExist(err) {
		t.Fatalf("archive should not exist: %v", err)
	}
}
