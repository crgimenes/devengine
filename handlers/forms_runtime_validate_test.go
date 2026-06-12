package handlers

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/db/sqlite"
)

func newValidateTestStore(t *testing.T) db.Store {
	t.Helper()
	s, err := sqlite.NewWithPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	err = sqlite.RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}
	return s
}

func setStorage(t *testing.T, s db.Store) {
	t.Helper()
	prev := db.Storage
	db.Storage = s
	t.Cleanup(func() { db.Storage = prev })
}

func textAttr(t *testing.T, s db.Store, entityID int64, machine, label string) db.EAVAttribute {
	t.Helper()
	a, err := s.CreateEAVAttribute(entityID, machine, label, "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(%s): %v", machine, err)
	}
	return *a
}

func TestValidateExprPassesOnEmpty(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Person", "person", "", "", "")
	attr := textAttr(t, s, et.ID, "name", "Name")

	el := db.FormElement{
		ElementKind:    "field",
		MachineName:    "name",
		EAVAttributeID: &attr.ID,
		ValidateExpr:   "", // empty: nothing to do
	}
	values := db.EAVRecordValues{"name": "anything"}

	errs, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, []db.EAVAttribute{attr}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("got %v, want empty", errs)
	}
}

func TestValidateExprRejectsWithStringResult(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Person", "person", "", "", "")
	attr := textAttr(t, s, et.ID, "name", "Name")

	el := db.FormElement{
		ElementKind:    "field",
		MachineName:    "name",
		EAVAttributeID: &attr.ID,
		Label:          "Nome",
		ValidateExpr:   `(if (= field:name "forbidden") "valor proibido" "")`,
	}
	values := db.EAVRecordValues{"name": "forbidden"}

	errs, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, []db.EAVAttribute{attr}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if errs["name"] != "valor proibido" {
		t.Fatalf("got %v", errs)
	}
}

func TestValidateExprPassesWhenStringEmpty(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Person", "person", "", "", "")
	attr := textAttr(t, s, et.ID, "name", "Name")

	el := db.FormElement{
		ElementKind:    "field",
		MachineName:    "name",
		EAVAttributeID: &attr.ID,
		ValidateExpr:   `(if (= field:name "forbidden") "no" "")`,
	}
	values := db.EAVRecordValues{"name": "allowed"}

	errs, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, []db.EAVAttribute{attr}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("got %v, want empty", errs)
	}
}

func TestValidateExprErrorGlobalWins(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Person", "person", "", "", "")
	attr := textAttr(t, s, et.ID, "name", "Name")

	el := db.FormElement{
		ElementKind:    "field",
		MachineName:    "name",
		EAVAttributeID: &attr.ID,
		Label:          "Nome",
		ValidateExpr:   `(set error "via global")`,
	}
	values := db.EAVRecordValues{"name": "x"}

	errs, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, []db.EAVAttribute{attr}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if errs["name"] != "via global" {
		t.Fatalf("got %v", errs)
	}
}

func TestValidateExprSkipsUIOnly(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	el := db.FormElement{
		ElementKind:    "divider",
		MachineName:    "sep",
		EAVAttributeID: nil,
		ValidateExpr:   `"this should be skipped"`,
	}
	errs, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, nil, nil)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("UI-only element ran validate_expr, got %v", errs)
	}
}

func TestValidateExprCanReadOtherFields(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Pair", "pair", "", "", "")
	a := textAttr(t, s, et.ID, "a", "A")
	b := textAttr(t, s, et.ID, "b", "B")

	// validate_expr on field 'b' references field 'a'
	el := db.FormElement{
		ElementKind:    "field",
		MachineName:    "b",
		EAVAttributeID: &b.ID,
		Label:          "B",
		ValidateExpr:   `(if (= field:a field:b) "a and b cannot be equal" "")`,
	}
	values := db.EAVRecordValues{"a": "same", "b": "same"}

	errs, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, []db.EAVAttribute{a, b}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if errs["b"] != "a and b cannot be equal" {
		t.Fatalf("got %v", errs)
	}
}

func TestValidateExprAggregatesAllErrors(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Multi", "multi", "", "", "")
	a := textAttr(t, s, et.ID, "a", "A")
	b := textAttr(t, s, et.ID, "b", "B")
	c := textAttr(t, s, et.ID, "c", "C")

	els := []db.FormElement{
		{ElementKind: "field", MachineName: "a", EAVAttributeID: &a.ID, Label: "Campo A",
			ValidateExpr: `"A invalid"`},
		{ElementKind: "field", MachineName: "b", EAVAttributeID: &b.ID, Label: "Campo B",
			ValidateExpr: `""`}, // valid
		{ElementKind: "field", MachineName: "c", EAVAttributeID: &c.ID, Label: "Campo C",
			ValidateExpr: `"C invalid"`},
	}
	values := db.EAVRecordValues{"a": "x", "b": "y", "c": "z"}

	errs, err := evaluateValidateExprs(context.Background(), nil, els, []db.EAVAttribute{a, b, c}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if errs["a"] != "A invalid" || errs["c"] != "C invalid" || len(errs) != 2 {
		t.Fatalf("got %v, want errors for a and c", errs)
	}
	joined := joinFieldErrors(els, errs)
	if joined != "Campo A: A invalid; Campo C: C invalid" {
		t.Fatalf("joined = %q", joined)
	}
}
