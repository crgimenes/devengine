package intp_test

import (
	"testing"

	"github.com/crgimenes/devengine/eav/ui"
	_ "github.com/crgimenes/devengine/eav/ui/intp"
)

func TestRegistered(t *testing.T) {
	p, ok := ui.Get("int")
	if !ok {
		t.Fatal("int plugin not registered")
	}
	if p.ID() != "int" {
		t.Fatalf("ID = %q", p.ID())
	}
}

func TestParseValid(t *testing.T) {
	p, _ := ui.Get("int")
	v, err := p.Parse(" 42 ", nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if v.(int64) != 42 {
		t.Fatalf("Parse = %v, want 42", v)
	}
}

func TestParseEmpty(t *testing.T) {
	p, _ := ui.Get("int")
	v, err := p.Parse("", nil)
	if err != nil {
		t.Fatalf("Parse(\"\"): %v", err)
	}
	if v.(int64) != 0 {
		t.Fatalf("Parse(\"\") = %v, want 0", v)
	}
}

func TestParseInvalid(t *testing.T) {
	p, _ := ui.Get("int")
	_, err := p.Parse("not a number", nil)
	if err == nil {
		t.Fatal("Parse(\"not a number\") expected error")
	}
}

func TestValidateBounds(t *testing.T) {
	p, _ := ui.Get("int")
	opts := p.ParseOptions(`{"min": 0, "max": 100}`)

	cases := []struct {
		in       int64
		hasError bool
	}{
		{-1, true},
		{0, false},
		{50, false},
		{100, false},
		{101, true},
	}
	for _, c := range cases {
		err := p.Validate(c.in, opts)
		if (err != nil) != c.hasError {
			t.Errorf("Validate(%d) error = %v, hasError = %v", c.in, err, c.hasError)
		}
	}
}
