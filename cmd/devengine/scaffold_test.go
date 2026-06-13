package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdir switches into dir for the duration of the test, restoring the old
// working directory afterwards. scaffold writes relative to the CWD.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestScaffoldWritesStarterFiles(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	err := scaffold("inventory.db")
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	filo, err := os.ReadFile(filepath.Join(dir, "init.filo"))
	if err != nil {
		t.Fatalf("read init.filo: %v", err)
	}
	if !strings.Contains(string(filo), `(set DBFile "inventory.db")`) {
		t.Fatalf("init.filo does not carry the db file: %s", filo)
	}

	main, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(main), "package main") ||
		!strings.Contains(string(main), "devengine/handlers") {
		t.Fatalf("main.go is not the starter app: %.80s", main)
	}
}

// scaffold must never clobber files a real app already owns.
func TestScaffoldKeepsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	const original = "// my own main\npackage main\n"
	err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(original), 0o600)
	if err != nil {
		t.Fatalf("seed main.go: %v", err)
	}

	err = scaffold("app.db")
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if string(got) != original {
		t.Fatalf("scaffold overwrote an existing main.go")
	}
}
