package textarea_test

import (
	"testing"

	"github.com/crgimenes/devengine/eav/ui"
	_ "github.com/crgimenes/devengine/eav/ui/textarea"
)

func TestRegistered(t *testing.T) {
	p, ok := ui.Get("textarea")
	if !ok {
		t.Fatal("textarea plugin not registered")
	}
	if p.ID() != "textarea" {
		t.Fatalf("ID = %q", p.ID())
	}
}

func TestParsePreservesInput(t *testing.T) {
	p, _ := ui.Get("textarea")
	in := "  line one\n  line two  "
	v, err := p.Parse(in, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if v != in {
		t.Fatalf("Parse = %q, want %q (raw preserved)", v, in)
	}
}

func TestValidateMaxLength(t *testing.T) {
	p, _ := ui.Get("textarea")
	opts := p.ParseOptions(`{"max_length": 5}`)
	if err := p.Validate("hello", opts); err != nil {
		t.Errorf("Validate(5 chars): %v", err)
	}
	if err := p.Validate("toolong", opts); err == nil {
		t.Error("Validate(7 chars) expected error")
	}
}
