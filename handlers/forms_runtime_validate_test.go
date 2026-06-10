package handlers

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/crgimenes/devengine/db"
)

func newValidateTestStore(t *testing.T) *db.SQLite {
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

func setStorage(t *testing.T, s *db.SQLite) {
	t.Helper()
	prev := db.Storage
	db.Storage = s
	t.Cleanup(func() { db.Storage = prev })
}

func textAttr(t *testing.T, s *db.SQLite, entityID int64, machine, label string) db.EAVAttribute {
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

	msg, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, []db.EAVAttribute{attr}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if msg != "" {
		t.Fatalf("got %q, want empty", msg)
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

	msg, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, []db.EAVAttribute{attr}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if msg != "Nome: valor proibido" {
		t.Fatalf("got %q", msg)
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

	msg, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, []db.EAVAttribute{attr}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if msg != "" {
		t.Fatalf("got %q, want empty", msg)
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

	msg, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, []db.EAVAttribute{attr}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if msg != "Nome: via global" {
		t.Fatalf("got %q", msg)
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
	msg, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, nil, nil)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if msg != "" {
		t.Fatalf("UI-only element ran validate_expr, got %q", msg)
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

	msg, err := evaluateValidateExprs(context.Background(), nil, []db.FormElement{el}, []db.EAVAttribute{a, b}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if msg != "B: a and b cannot be equal" {
		t.Fatalf("got %q", msg)
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

	msg, err := evaluateValidateExprs(context.Background(), nil, els, []db.EAVAttribute{a, b, c}, values)
	if err != nil {
		t.Fatalf("evaluateValidateExprs: %v", err)
	}
	if msg != "Campo A: A invalid; Campo C: C invalid" {
		t.Fatalf("got %q, want both errors joined by '; '", msg)
	}
}
