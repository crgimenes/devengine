package decimal_test

import (
	"math"
	"testing"

	"github.com/crgimenes/devengine/eav/ui"
	_ "github.com/crgimenes/devengine/eav/ui/decimal"
)

func TestRegistered(t *testing.T) {
	p, ok := ui.Get("decimal")
	if !ok {
		t.Fatal("decimal plugin not registered")
	}
	if p.ID() != "decimal" {
		t.Fatalf("ID = %q", p.ID())
	}
}

func TestParseValid(t *testing.T) {
	p, _ := ui.Get("decimal")
	v, err := p.Parse(" 3.14 ", nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if v.(float64) != 3.14 {
		t.Fatalf("Parse = %v, want 3.14", v)
	}
}

func TestParseEmpty(t *testing.T) {
	p, _ := ui.Get("decimal")
	v, err := p.Parse("", nil)
	if err != nil {
		t.Fatalf("Parse(\"\"): %v", err)
	}
	if v.(float64) != 0 {
		t.Fatalf("Parse(\"\") = %v, want 0", v)
	}
}

func TestParseInvalid(t *testing.T) {
	p, _ := ui.Get("decimal")
	_, err := p.Parse("abc", nil)
	if err == nil {
		t.Fatal("Parse(\"abc\") expected error")
	}
}

func TestValidateRejectsNaN(t *testing.T) {
	p, _ := ui.Get("decimal")
	err := p.Validate(math.NaN(), nil)
	if err == nil {
		t.Fatal("Validate(NaN) expected error")
	}
}

func TestValidateBounds(t *testing.T) {
	p, _ := ui.Get("decimal")
	opts := p.ParseOptions(`{"min": 0.0, "max": 1.0}`)
	cases := []struct {
		in       float64
		hasError bool
	}{
		{-0.1, true},
		{0.0, false},
		{0.5, false},
		{1.0, false},
		{1.1, true},
	}
	for _, c := range cases {
		err := p.Validate(c.in, opts)
		if (err != nil) != c.hasError {
			t.Errorf("Validate(%g) error = %v, hasError = %v", c.in, err, c.hasError)
		}
	}
}
