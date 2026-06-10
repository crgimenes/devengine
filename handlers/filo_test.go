package handlers

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/db"
)

func newFiloTestStore(t *testing.T) *db.SQLite {
	t.Helper()
	s, err := db.NewWithPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	err = db.RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}
	return s
}

func setupFiloTest(t *testing.T) (*Handlers, *db.User) {
	t.Helper()
	s := newFiloTestStore(t)
	t.Cleanup(func() { s.Close() })

	prev := db.Storage
	db.Storage = s
	t.Cleanup(func() { db.Storage = prev })

	user := &db.User{ID: 1, Username: "admin", Sysop: true}
	h := &Handlers{}
	return h, user
}

func TestRunFiloScriptReturnsValue(t *testing.T) {
	h, user := setupFiloTest(t)

	got := h.runFiloScript(user, `(str-concat "hello, " field:name)`, `{"field:name": "Alice"}`)
	if got.SysError != "" {
		t.Fatalf("SysError: %s", got.SysError)
	}
	if got.UserError != "" {
		t.Fatalf("UserError: %s", got.UserError)
	}
	if got.Result != "hello, Alice" {
		t.Fatalf("Result = %q, want %q", got.Result, "hello, Alice")
	}
	if got.ResultKind != "string" {
		t.Fatalf("ResultKind = %q", got.ResultKind)
	}
}

func TestRunFiloScriptEmptyScript(t *testing.T) {
	h, user := setupFiloTest(t)

	got := h.runFiloScript(user, "   ", "")
	if got.UserError == "" {
		t.Fatal("expected UserError for empty script")
	}
}

func TestRunFiloScriptInvalidGlobalsJSON(t *testing.T) {
	h, user := setupFiloTest(t)

	got := h.runFiloScript(user, `"x"`, `{not json}`)
	if got.UserError == "" || !strings.Contains(got.UserError, "JSON") {
		t.Fatalf("UserError = %q, want JSON message", got.UserError)
	}
}

func TestRunFiloScriptErrorGlobalSurfaces(t *testing.T) {
	h, user := setupFiloTest(t)

	got := h.runFiloScript(user, `(set error "manual rejection")`, "")
	if got.UserError != "manual rejection" {
		t.Fatalf("UserError = %q", got.UserError)
	}
}

func TestRunFiloScriptSysErrorOnSyntax(t *testing.T) {
	h, user := setupFiloTest(t)

	got := h.runFiloScript(user, `(this is not valid`, "")
	if got.SysError == "" {
		t.Fatal("expected SysError for syntactically broken script")
	}
}

func TestRunFiloScriptListsGlobalsBack(t *testing.T) {
	h, user := setupFiloTest(t)

	got := h.runFiloScript(user, `(set greeting "hi")`, `{"field:name": "Alice"}`)
	if got.SysError != "" {
		t.Fatalf("SysError: %s", got.SysError)
	}
	keys := map[string]string{}
	for _, g := range got.Globals {
		keys[g.Key] = g.Value
	}
	if keys["field:name"] != "Alice" {
		t.Errorf("field:name not preserved: %+v", keys)
	}
	if keys["greeting"] != "hi" {
		t.Errorf("greeting not in globals: %+v", keys)
	}
}
