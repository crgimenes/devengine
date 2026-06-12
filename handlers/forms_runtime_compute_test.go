package handlers

import (
	"context"
	"testing"

	"github.com/crgimenes/devengine/db"
)

func computedTextAttr(t *testing.T, s db.Store, entityID int64, machine, label, expr string) db.EAVAttribute {
	t.Helper()
	a, err := s.CreateEAVAttribute(entityID, machine, label, "", "TEXT", false, false, false, nil, true, expr, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(%s): %v", machine, err)
	}
	return *a
}

func computedIntAttr(t *testing.T, s db.Store, entityID int64, machine, label, expr string) db.EAVAttribute {
	t.Helper()
	a, err := s.CreateEAVAttribute(entityID, machine, label, "", "INT", false, false, false, nil, true, expr, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(%s): %v", machine, err)
	}
	return *a
}

func TestComputedExprOverwritesValue(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Order", "order", "", "", "")
	a := textAttr(t, s, et.ID, "code", "Code")
	b := computedTextAttr(t, s, et.ID, "label", "Label", `(str-concat "order-" field:code)`)

	values := db.EAVRecordValues{
		"code":  "42",
		"label": "user submitted this but it should be ignored",
	}
	got, err := applyComputedExprs(context.Background(), nil, []db.EAVAttribute{a, b}, values)
	if err != nil {
		t.Fatalf("applyComputedExprs: %v", err)
	}
	if got["label"] != "order-42" {
		t.Fatalf("label = %v, want %q", got["label"], "order-42")
	}
	if got["code"] != "42" {
		t.Fatalf("code mutated: %v", got["code"])
	}
}

func TestComputedExprSkipsNonComputed(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Doc", "doc", "", "", "")
	a := textAttr(t, s, et.ID, "name", "Name") // not computed

	values := db.EAVRecordValues{"name": "alpha"}
	got, err := applyComputedExprs(context.Background(), nil, []db.EAVAttribute{a}, values)
	if err != nil {
		t.Fatalf("applyComputedExprs: %v", err)
	}
	if got["name"] != "alpha" {
		t.Fatalf("name changed: %v", got["name"])
	}
}

func TestComputedExprIntResult(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Numbers", "numbers", "", "", "")
	a, err := s.CreateEAVAttribute(et.ID, "x", "X", "", "INT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(x): %v", err)
	}
	b := computedIntAttr(t, s, et.ID, "double", "Double", `(* field:x 2)`)

	values := db.EAVRecordValues{"x": int64(7), "double": int64(0)}
	got, err := applyComputedExprs(context.Background(), nil, []db.EAVAttribute{*a, b}, values)
	if err != nil {
		t.Fatalf("applyComputedExprs: %v", err)
	}
	if got["double"] != int64(14) {
		t.Fatalf("double = %v (%T), want int64(14)", got["double"], got["double"])
	}
}

func TestComputedExprChainsThroughOrder(t *testing.T) {
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Chain", "chain", "", "", "")
	base := textAttr(t, s, et.ID, "base", "Base")
	first := computedTextAttr(t, s, et.ID, "first", "First", `(str-concat field:base "-1")`)
	second := computedTextAttr(t, s, et.ID, "second", "Second", `(str-concat field:first "-2")`)

	values := db.EAVRecordValues{"base": "abc", "first": "", "second": ""}
	got, err := applyComputedExprs(context.Background(), nil, []db.EAVAttribute{base, first, second}, values)
	if err != nil {
		t.Fatalf("applyComputedExprs: %v", err)
	}
	if got["first"] != "abc-1" {
		t.Fatalf("first = %v", got["first"])
	}
	if got["second"] != "abc-1-2" {
		t.Fatalf("second = %v (computed should chain)", got["second"])
	}
}
