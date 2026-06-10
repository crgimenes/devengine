package datetime_test

import (
	"strings"
	"testing"
	"time"

	"github.com/crgimenes/devengine/eav/ui"
	_ "github.com/crgimenes/devengine/eav/ui/datetime"
)

func TestRegistered(t *testing.T) {
	p, ok := ui.Get("datetime")
	if !ok {
		t.Fatal("datetime plugin not registered")
	}
	if p.ID() != "datetime" {
		t.Fatalf("ID = %q", p.ID())
	}
}

func TestParseLocalToUTC(t *testing.T) {
	p, _ := ui.Get("datetime")

	cases := []string{
		"2025-12-09T14:30",
		"2025-12-09T14:30:45",
		"2025-12-09T14:30:45Z",
	}
	for _, in := range cases {
		v, err := p.Parse(in, nil)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)
			continue
		}
		s := v.(string)
		_, err = time.Parse(time.RFC3339, s)
		if err != nil {
			t.Errorf("Parse(%q) = %q; not RFC3339: %v", in, s, err)
		}
		if !strings.HasSuffix(s, "Z") && !strings.Contains(s, "+00:00") {
			t.Errorf("Parse(%q) = %q; not UTC", in, s)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	p, _ := ui.Get("datetime")
	v, err := p.Parse("", nil)
	if err != nil {
		t.Fatalf("Parse(\"\"): %v", err)
	}
	if v != "" {
		t.Fatalf("Parse(\"\") = %q, want empty", v)
	}
}

func TestParseInvalid(t *testing.T) {
	p, _ := ui.Get("datetime")
	_, err := p.Parse("not a date", nil)
	if err == nil {
		t.Fatal("Parse(\"not a date\") expected error")
	}
}

func TestValidateRange(t *testing.T) {
	p, _ := ui.Get("datetime")
	opts := p.ParseOptions(`{"min": "2025-01-01T00:00", "max": "2025-12-31T23:59"}`)

	inside, _ := p.Parse("2025-06-15T12:00", nil)
	if err := p.Validate(inside, opts); err != nil {
		t.Errorf("Validate(inside): %v", err)
	}

	before, _ := p.Parse("2024-12-31T23:59", nil)
	if err := p.Validate(before, opts); err == nil {
		t.Error("Validate(before) expected error")
	}

	after, _ := p.Parse("2026-01-01T00:00", nil)
	if err := p.Validate(after, opts); err == nil {
		t.Error("Validate(after) expected error")
	}
}
