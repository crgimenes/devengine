package text_test

import (
	"strings"
	"testing"

	"github.com/crgimenes/devengine/eav/ui"
	_ "github.com/crgimenes/devengine/eav/ui/text"
)

func TestRegistered(t *testing.T) {
	p, ok := ui.Get("text")
	if !ok {
		t.Fatal("text plugin not registered")
	}
	if p.ID() != "text" {
		t.Fatalf("ID = %q", p.ID())
	}
	if !p.HasPersistence() {
		t.Fatal("HasPersistence = false")
	}
}

func TestParseTrimsWhitespace(t *testing.T) {
	p, _ := ui.Get("text")
	v, err := p.Parse("  hello  ", p.ParseOptions(""))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if v != "hello" {
		t.Fatalf("Parse = %q, want %q", v, "hello")
	}
}

func TestValidateBounds(t *testing.T) {
	p, _ := ui.Get("text")
	opts := p.ParseOptions(`{"min_length": 3, "max_length": 5}`)

	cases := []struct {
		in       string
		hasError bool
	}{
		{"ab", true},
		{"abc", false},
		{"abcde", false},
		{"abcdef", true},
	}
	for _, c := range cases {
		err := p.Validate(c.in, opts)
		if (err != nil) != c.hasError {
			t.Errorf("Validate(%q) error = %v, hasError = %v", c.in, err, c.hasError)
		}
	}
}

func TestValidateRejectsNonString(t *testing.T) {
	p, _ := ui.Get("text")
	err := p.Validate(42, p.ParseOptions(""))
	if err == nil || !strings.Contains(err.Error(), "string") {
		t.Fatalf("Validate(42) error = %v, want non-nil mentioning string", err)
	}
}
