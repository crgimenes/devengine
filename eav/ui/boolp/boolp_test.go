package boolp_test

import (
	"testing"

	"github.com/crgimenes/devengine/eav/ui"
	_ "github.com/crgimenes/devengine/eav/ui/boolp"
)

func TestParseAccepts(t *testing.T) {
	p, ok := ui.Get("bool")
	if !ok {
		t.Fatal("bool plugin not registered")
	}

	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"false", false},
		{"0", false},
		{"off", false},
		{"no", false},
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"on", true},
		{"yes", true},
	}
	for _, c := range cases {
		v, err := p.Parse(c.in, nil)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if v.(bool) != c.want {
			t.Errorf("Parse(%q) = %v, want %v", c.in, v, c.want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	p, _ := ui.Get("bool")
	_, err := p.Parse("maybe", nil)
	if err == nil {
		t.Fatal("Parse(\"maybe\") expected error")
	}
}

func TestPluginMetadata(t *testing.T) {
	p, ok := ui.Get("bool")
	if !ok {
		t.Fatal("bool plugin not registered")
	}
	if p.ID() != "bool" {
		t.Fatalf("ID = %q", p.ID())
	}
	if kinds := p.PrimitiveKinds(); len(kinds) != 1 || kinds[0] != "BOOL" {
		t.Fatalf("PrimitiveKinds = %v", kinds)
	}
	if !p.HasPersistence() || !p.SupportsReadOnly() {
		t.Fatal("persistence/readonly flags wrong")
	}
	if d := p.Defaults(); d["style"] != "select" {
		t.Fatalf("Defaults = %v", d)
	}
	if opts := p.ParseOptions(`{"style": "switch"}`); opts != nil {
		t.Fatalf("ParseOptions = %v, want nil (bool has no options)", opts)
	}
}

func TestValidateRequiresBool(t *testing.T) {
	p, _ := ui.Get("bool")
	if err := p.Validate(true, nil); err != nil {
		t.Fatalf("Validate(true) = %v", err)
	}
	if err := p.Validate("yes", nil); err == nil {
		t.Fatal("non-bool accepted")
	}
}
