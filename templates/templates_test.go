package templates

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/eav/ui"
)

func TestGetGroupDefaults(t *testing.T) {
	tests := []struct {
		name        string
		elementKind string
		expectedLen int
		checkKey    string
		checkValue  any
	}{
		{
			name:        "group defaults",
			elementKind: "group",
			expectedLen: 2,
			checkKey:    "show_legend",
			checkValue:  true,
		},
		{
			name:        "accordion defaults",
			elementKind: "accordion",
			expectedLen: 2,
			checkKey:    "expanded_by_default",
			checkValue:  true,
		},
		{
			name:        "card defaults",
			elementKind: "card",
			expectedLen: 2,
			checkKey:    "show_header",
			checkValue:  true,
		},
		{
			name:        "tabs defaults",
			elementKind: "tabs",
			expectedLen: 2,
			checkKey:    "active_tab",
			checkValue:  0,
		},
		{
			name:        "carousel defaults",
			elementKind: "carousel",
			expectedLen: 4,
			checkKey:    "show_controls",
			checkValue:  true,
		},
		{
			name:        "unknown type returns empty map",
			elementKind: "unknown",
			expectedLen: 0,
			checkKey:    "",
			checkValue:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defaults := getGroupDefaults(tc.elementKind)

			if len(defaults) != tc.expectedLen {
				t.Errorf("expected %d keys, got %d", tc.expectedLen, len(defaults))
			}

			if tc.checkKey != "" {
				val, ok := defaults[tc.checkKey]
				if !ok {
					t.Errorf("expected key %q not found", tc.checkKey)
				} else if val != tc.checkValue {
					t.Errorf("expected %q = %v, got %v", tc.checkKey, tc.checkValue, val)
				}
			}
		})
	}
}

func TestParseGroupMetaIntegration(t *testing.T) {
	// Get the parseGroupMeta function via funcMap - we'll test the logic directly
	tests := []struct {
		name        string
		jsonStr     string
		elementKind string
		checkKey    string
		expected    any
	}{
		{
			name:        "empty JSON uses defaults",
			jsonStr:     "",
			elementKind: "accordion",
			checkKey:    "expanded_by_default",
			expected:    true,
		},
		{
			name:        "invalid JSON uses defaults",
			jsonStr:     "{invalid}",
			elementKind: "accordion",
			checkKey:    "expanded_by_default",
			expected:    true,
		},
		{
			name:        "valid JSON overrides defaults",
			jsonStr:     `{"expanded_by_default": false}`,
			elementKind: "accordion",
			checkKey:    "expanded_by_default",
			expected:    false,
		},
		{
			name:        "partial JSON merges with defaults",
			jsonStr:     `{"show_controls": false}`,
			elementKind: "carousel",
			checkKey:    "show_indicators",
			expected:    true, // from defaults
		},
		{
			name:        "carousel interval override",
			jsonStr:     `{"interval": 3000}`,
			elementKind: "carousel",
			checkKey:    "interval",
			expected:    float64(3000), // JSON numbers are float64
		},
	}

	// Simulating the parseGroupMeta logic inline for testing
	parseGroupMeta := func(jsonStr string, elementKind string) map[string]any {
		defaults := getGroupDefaults(elementKind)
		if jsonStr == "" {
			return defaults
		}
		var meta map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
			return defaults
		}
		maps.Copy(defaults, meta)
		return defaults
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseGroupMeta(tc.jsonStr, tc.elementKind)

			val, ok := result[tc.checkKey]
			if !ok {
				t.Errorf("expected key %q not found in result", tc.checkKey)
				return
			}

			if val != tc.expected {
				t.Errorf("expected %q = %v (%T), got %v (%T)",
					tc.checkKey, tc.expected, tc.expected, val, val)
			}
		})
	}
}

func TestGetFieldDefaults(t *testing.T) {
	// Plugin-owned kinds delegate to the registry (per-plugin values are
	// asserted in eav/ui/defaults); media kinds and unknown ids fall back.
	ui.Register("stubkind", func() ui.FieldUI { return stubDefaultsPlugin{} })

	got := getFieldDefaults("stubkind")
	if len(got) != 1 || got["answer"] != 42 {
		t.Fatalf("registered plugin defaults not used: %v", got)
	}

	for kind, key := range map[string]string{
		"image": "alt_text",
		"video": "controls",
		"audio": "controls",
	} {
		d := getFieldDefaults(kind)
		if _, ok := d[key]; !ok {
			t.Errorf("media fallback %s missing key %s: %v", kind, key, d)
		}
	}

	if d := getFieldDefaults("unknown"); len(d) != 0 {
		t.Errorf("unknown kind must return empty defaults: %v", d)
	}
}

type stubDefaultsPlugin struct{}

func (stubDefaultsPlugin) ID() string                     { return "stubkind" }
func (stubDefaultsPlugin) PrimitiveKinds() []string       { return []string{"TEXT"} }
func (stubDefaultsPlugin) HasPersistence() bool           { return true }
func (stubDefaultsPlugin) SupportsReadOnly() bool         { return true }
func (stubDefaultsPlugin) Defaults() map[string]any       { return map[string]any{"answer": 42} }
func (stubDefaultsPlugin) ParseOptions(string) any        { return nil }
func (stubDefaultsPlugin) Parse(string, any) (any, error) { return nil, nil }
func (stubDefaultsPlugin) Validate(any, any) error        { return nil }

func TestParseFieldMetaIntegration(t *testing.T) {
	ui.Register("stubkind", func() ui.FieldUI { return stubDefaultsPlugin{} })
	parseFieldMeta := func(jsonStr string, uiKind string) map[string]any {
		defaults := getFieldDefaults(uiKind)
		if jsonStr == "" {
			return defaults
		}
		var meta map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
			return defaults
		}
		maps.Copy(defaults, meta)
		return defaults
	}

	tests := []struct {
		name     string
		jsonStr  string
		uiKind   string
		checkKey string
		expected any
	}{
		{"empty JSON uses plugin defaults", "", "stubkind", "answer", 42},
		{"invalid JSON uses plugin defaults", "{bad}", "stubkind", "answer", 42},
		{"stored JSON overrides defaults", "{\"answer\": 7}", "stubkind", "answer", float64(7)},
		{"media fallback", "", "image", "alt_text", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseFieldMeta(tc.jsonStr, tc.uiKind)

			val, ok := result[tc.checkKey]
			if !ok {
				t.Errorf("expected key %q not found", tc.checkKey)
				return
			}

			if val != tc.expected {
				t.Errorf("expected %q = %v (%T), got %v (%T)",
					tc.checkKey, tc.expected, tc.expected, val, val)
			}
		})
	}
}

func TestRenderFieldDispatches(t *testing.T) {
	ensureTemplatesLoaded()

	data := map[string]any{
		"Meta":     map[string]any{},
		"Name":     "alpha",
		"Value":    "hello",
		"Required": false,
		"Readonly": false,
	}

	got, err := renderField("field_text", data)
	if err != nil {
		t.Fatalf("renderField(field_text): %v", err)
	}
	if !strings.Contains(string(got), `name="alpha"`) {
		t.Errorf("output missing name=alpha: %s", got)
	}
	if !strings.Contains(string(got), `value="hello"`) {
		t.Errorf("output missing value=hello: %s", got)
	}
}

func TestRenderFieldFallsBackToText(t *testing.T) {
	ensureTemplatesLoaded()

	data := map[string]any{
		"Meta":     map[string]any{},
		"Name":     "alpha",
		"Value":    "fallback",
		"Required": false,
		"Readonly": false,
	}

	got, err := renderField("field_does_not_exist", data)
	if err != nil {
		t.Fatalf("renderField(unknown): %v", err)
	}
	if !strings.Contains(string(got), `value="fallback"`) {
		t.Errorf("fallback to field_text did not render the value: %s", got)
	}
}
