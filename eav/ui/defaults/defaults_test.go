package defaults

import (
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
		"decimal":   {5, "decimalPlaces"},
		"bool":      {1, "style"},
		"datetime":  {3, "includeTime"},
		"select":    {3, "options"},
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
