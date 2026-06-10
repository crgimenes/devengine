package handlers

import (
	"testing"

	"github.com/crgimenes/devengine/db"
)

func TestApplyPrefillSetsValueByMachineName(t *testing.T) {
	attrs := []db.EAVAttribute{
		{MachineName: "customer_ref", PrimitiveKind: "TEXT"},
		{MachineName: "qty", PrimitiveKind: "INT"},
	}
	values := map[string]any{}

	applyPrefill(values, attrs, []string{"customer_ref=abc-123", "qty=7"})

	if values["customer_ref"] != "abc-123" {
		t.Errorf("customer_ref = %v", values["customer_ref"])
	}
	if values["qty"] != int64(7) {
		t.Errorf("qty = %v (%T)", values["qty"], values["qty"])
	}
}

func TestApplyPrefillIgnoresUnknownAttr(t *testing.T) {
	attrs := []db.EAVAttribute{{MachineName: "name", PrimitiveKind: "TEXT"}}
	values := map[string]any{}

	applyPrefill(values, attrs, []string{"ghost=x"})

	if _, ok := values["ghost"]; ok {
		t.Fatal("unknown attr was applied")
	}
}

func TestApplyPrefillIgnoresMalformedPair(t *testing.T) {
	attrs := []db.EAVAttribute{{MachineName: "name", PrimitiveKind: "TEXT"}}
	values := map[string]any{}

	applyPrefill(values, attrs, []string{"no-equal-sign"})

	if len(values) != 0 {
		t.Fatalf("got %v, want empty", values)
	}
}

func TestApplyPrefillNoOpWhenRawEmpty(t *testing.T) {
	values := map[string]any{"existing": "untouched"}
	applyPrefill(values, nil, nil)
	if values["existing"] != "untouched" {
		t.Fatal("applyPrefill mutated values when no raw provided")
	}
}

func TestCoercePrefillTypes(t *testing.T) {
	cases := []struct {
		primitive string
		in        string
		want      any
	}{
		{"BOOL", "true", true},
		{"BOOL", "1", true},
		{"BOOL", "false", false},
		{"INT", "42", int64(42)},
		{"INT", "garbage", "garbage"},
		{"REAL", "3.14", 3.14},
		{"TEXT", "hello", "hello"},
		{"DATETIME", "2026-01-01T12:00:00Z", "2026-01-01T12:00:00Z"},
	}
	for _, tc := range cases {
		got := coercePrefill(tc.primitive, tc.in)
		if got != tc.want {
			t.Errorf("coercePrefill(%q, %q) = %v (%T), want %v (%T)",
				tc.primitive, tc.in, got, got, tc.want, tc.want)
		}
	}
}
