package handlers

import (
	"testing"

	"github.com/crgimenes/devengine/db"
	_ "github.com/crgimenes/devengine/eav/ui/defaults"
)

func TestDefaultUIKind(t *testing.T) {
	tests := []struct {
		primitive string
		want      string
	}{
		{"BOOL", "bool"},
		{"INT", "int"},
		{"REAL", "decimal"},
		{"DATETIME", "datetime"},
		{"TEXT", "text"},
		{"", "text"},
		{"unknown", "text"},
	}
	for _, tc := range tests {
		t.Run(tc.primitive, func(t *testing.T) {
			got := defaultUIKind(tc.primitive)
			if got != tc.want {
				t.Fatalf("defaultUIKind(%q) = %q, want %q", tc.primitive, got, tc.want)
			}
		})
	}
}

// A field bound to an EAV attribute often has no explicit ui_kind. The save
// path must still normalize the value through the primitive's plugin so a
// datetime is stored as RFC3339 UTC rather than the raw datetime-local string.
func TestParseElementValueDatetimeWithoutUIKind(t *testing.T) {
	el := db.FormElement{UIKind: ""}
	attr := &db.EAVAttribute{PrimitiveKind: "DATETIME"}

	got, err := parseElementValue(el, attr, "2026-06-15T18:00")
	if err != nil {
		t.Fatalf("parseElementValue: %v", err)
	}
	if got != "2026-06-15T18:00:00Z" {
		t.Fatalf("got %v, want RFC3339 UTC 2026-06-15T18:00:00Z", got)
	}
}
