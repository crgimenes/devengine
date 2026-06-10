package selectp_test

import (
	"testing"

	"github.com/crgimenes/devengine/eav/ui"
	_ "github.com/crgimenes/devengine/eav/ui/selectp"
)

func TestRegistered(t *testing.T) {
	p, ok := ui.Get("select")
	if !ok {
		t.Fatal("select plugin not registered")
	}
	if p.ID() != "select" {
		t.Fatalf("ID = %q", p.ID())
	}
}

func TestParseTextKindDefault(t *testing.T) {
	p, _ := ui.Get("select")
	v, err := p.Parse("alpha", nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if v != "alpha" {
		t.Fatalf("Parse = %q, want \"alpha\"", v)
	}
}

func TestParseIntKind(t *testing.T) {
	p, _ := ui.Get("select")
	opts := p.ParseOptions(`{"kind": "int"}`)
	v, err := p.Parse("42", opts)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if v.(int64) != 42 {
		t.Fatalf("Parse = %v, want 42", v)
	}
}

func TestParseIntKindInvalid(t *testing.T) {
	p, _ := ui.Get("select")
	opts := p.ParseOptions(`{"kind": "int"}`)
	_, err := p.Parse("abc", opts)
	if err == nil {
		t.Fatal("Parse(\"abc\") with kind=int expected error")
	}
}

func TestValidateInList(t *testing.T) {
	p, _ := ui.Get("select")
	opts := p.ParseOptions(`{"options": [{"value":"a"},{"value":"b"}]}`)
	if err := p.Validate("a", opts); err != nil {
		t.Errorf("Validate(\"a\"): %v", err)
	}
	if err := p.Validate("c", opts); err == nil {
		t.Error("Validate(\"c\") expected error")
	}
}

func TestValidateEmptyRequiresAllow(t *testing.T) {
	p, _ := ui.Get("select")
	opts := p.ParseOptions(`{"options": [{"value":"a"}]}`)
	if err := p.Validate("", opts); err == nil {
		t.Error("Validate(\"\") expected error when allow_empty is false")
	}

	opts = p.ParseOptions(`{"options": [{"value":"a"}], "allow_empty": true}`)
	if err := p.Validate("", opts); err != nil {
		t.Errorf("Validate(\"\") with allow_empty=true: %v", err)
	}
}

func TestValidateIntInList(t *testing.T) {
	p, _ := ui.Get("select")
	opts := p.ParseOptions(`{"kind":"int","options":[{"value":"1"},{"value":"2"}]}`)
	if err := p.Validate(int64(1), opts); err != nil {
		t.Errorf("Validate(1): %v", err)
	}
	if err := p.Validate(int64(3), opts); err == nil {
		t.Error("Validate(3) expected error")
	}
}
