package defaults

import (
	"strings"
	"testing"

	"github.com/crgimenes/devengine/eav/ui"
)

// Every canonical plugin owns its ui_meta defaults. The expected key counts
// moved here from the old templates.getFieldDefaults switch when the values
// became plugin-owned.
func TestCanonicalPluginDefaults(t *testing.T) {
	want := map[string]struct {
		keys     int
		checkKey string
	}{
		"text":      {4, "placeholder"},
		"textarea":  {3, "rows"},
		"int":       {4, "step"},
		"decimal":   {5, "decimal_places"},
		"bool":      {1, "style"},
		"datetime":  {3, "include_time"},
		"select":    {2, "options"},
		"reference": {0, ""},
		"subform":   {0, ""},
	}

	for id, exp := range want {
		p, ok := ui.Get(id)
		if !ok {
			t.Errorf("plugin %s not registered", id)
			continue
		}
		d := p.Defaults()
		if d == nil {
			t.Errorf("plugin %s: Defaults() must not return nil", id)
			continue
		}
		if len(d) != exp.keys {
			t.Errorf("plugin %s: %d default keys, want %d (%v)", id, len(d), exp.keys, d)
		}
		if exp.checkKey != "" {
			if _, ok := d[exp.checkKey]; !ok {
				t.Errorf("plugin %s: default key %q missing", id, exp.checkKey)
			}
		}
	}
}

// ui_meta_json keys are snake_case across defaults, templates, and the
// plugins' Options structs. A camelCase default silently diverges from the
// Options json tags and the value never reaches Parse/Validate — that bug
// shipped once (allowEmpty vs allow_empty) and this guards against it.
func TestDefaultsKeysAreSnakeCase(t *testing.T) {
	for _, id := range []string{
		"text", "textarea", "int", "decimal", "bool",
		"datetime", "select", "reference", "subform",
	} {
		p, ok := ui.Get(id)
		if !ok {
			t.Errorf("plugin %s not registered", id)
			continue
		}
		for key := range p.Defaults() {
			if key != strings.ToLower(key) {
				t.Errorf("plugin %s: default key %q is not snake_case", id, key)
			}
		}
	}
}
