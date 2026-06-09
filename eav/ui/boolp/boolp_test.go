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
