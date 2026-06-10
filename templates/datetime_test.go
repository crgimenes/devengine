package templates

import "testing"

func TestHTMLDatetime(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		includeTime any
		want        string
	}{
		{"rfc3339 with Z", "2026-06-15T18:00:00Z", true, "2026-06-15T18:00"},
		{"html5 local no seconds", "2026-06-15T18:00", true, "2026-06-15T18:00"},
		{"html5 local with seconds", "2026-06-15T18:00:05", true, "2026-06-15T18:00"},
		{"sqlite space layout", "2026-06-15 18:00:05", true, "2026-06-15T18:00"},
		{"date only as datetime", "2026-06-15", true, "2026-06-15T00:00"},
		{"date only field", "2026-06-15T18:00:00Z", false, "2026-06-15"},
		{"empty", "", true, ""},
		{"unparseable", "not a date", true, ""},
		{"non-string", 42, true, ""},
		{"includeTime nil defaults to datetime", "2026-06-15T18:00:00Z", nil, "2026-06-15T18:00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := htmlDatetime(tc.value, tc.includeTime)
			if got != tc.want {
				t.Fatalf("htmlDatetime(%v, %v) = %q, want %q", tc.value, tc.includeTime, got, tc.want)
			}
		})
	}
}
