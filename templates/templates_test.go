package templates

import (
	"encoding/json"
	"testing"
)

func TestGetGroupDefaults(t *testing.T) {
	tests := []struct {
		name        string
		elementKind string
		expectedLen int
		checkKey    string
		checkValue  interface{}
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
		expected    interface{}
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
	parseGroupMeta := func(jsonStr string, elementKind string) map[string]interface{} {
		defaults := getGroupDefaults(elementKind)
		if jsonStr == "" {
			return defaults
		}
		var meta map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
			return defaults
		}
		for k, v := range meta {
			defaults[k] = v
		}
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
