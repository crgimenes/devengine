package eav

import (
	"testing"

	"github.com/crgimenes/devengine/db"
)

func TestIsGroupingFieldFlag(t *testing.T) {
	field := db.EAVField{IsGroupingField: true}
	if !isGroupingField(field) {
		t.Fatalf("expected grouping flag to be honored")
	}

	field.IsGroupingField = false
	if isGroupingField(field) {
		t.Fatalf("expected non-grouping flag to be false")
	}
}

func TestParentGroupOptionsIncludesGroupings(t *testing.T) {
	fields := []db.EAVField{
		{MachineName: "root", Label: "Root", UIKind: "text"},
		{MachineName: "box", Label: "Box", UIKind: "group", IsGroupingField: true},
		{MachineName: "acc", Label: "Accordion", UIKind: "group_accordion", IsGroupingField: true},
	}

	opts := parentGroupOptions(fields, "")
	if len(opts) != 2 {
		t.Fatalf("expected 2 parent options, got %d", len(opts))
	}

	if opts[0].Value != "box" || opts[1].Value != "acc" {
		t.Fatalf("unexpected options order %+v", opts)
	}
}

func TestIsValidParentSelectionRequiresGroupingFlag(t *testing.T) {
	fields := []db.EAVField{
		{MachineName: "child", Label: "Child", UIKind: "text"},
		{MachineName: "not_group", Label: "Not Group", UIKind: "group"},
		{MachineName: "grouping", Label: "Grouping", UIKind: "group", IsGroupingField: true},
	}

	if isValidParentSelection(fields, "child", "not_group") {
		t.Fatalf("non-grouping field should not be valid parent")
	}

	if !isValidParentSelection(fields, "child", "grouping") {
		t.Fatalf("grouping field should be valid parent")
	}

	if isValidParentSelection(fields, "child", "child") {
		t.Fatalf("self selection must be invalid")
	}
}

func TestRecommendedPrimitiveKindForUIKind(t *testing.T) {
	cases := map[string]string{
		"text":            "TEXT",
		"textarea":        "TEXT",
		"integer":         "INT",
		"decimal":         "FLOAT",
		"boolean":         "BOOL",
		"group":           "-",
		"group_accordion": "-",
		"divider":         "TEXT",
		"unknown":         "TEXT",
	}

	for uiKind, expected := range cases {
		kind := recommendedPrimitiveKindForUIKind(uiKind)
		if kind != expected {
			t.Fatalf("ui kind %s expected %s got %s", uiKind, expected, kind)
		}
	}
}

func TestRenderUIOptionsCarriesPrimitiveKind(t *testing.T) {
	view, err := renderUIOptions(1, nil, "integer", false, false, false)
	if err != nil {
		t.Fatalf("renderUIOptions error: %v", err)
	}

	if view.PrimitiveKind != "INT" {
		t.Fatalf("expected recommended primitive kind, got %s", view.PrimitiveKind)
	}
}
