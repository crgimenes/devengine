package templates

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"strings"
	"sync"
)

var (
	extraFS        []fs.FS
	appTemplatesFS fs.FS
	loadOnce       sync.Once
	tpl            *template.Template
)

// RegisterFS allows feature packages to contribute template file systems
// before the first render.
func RegisterFS(fsys fs.FS) {
	extraFS = append(extraFS, fsys)
}

// SetAppTemplatesFS configures an optional filesystem containing
// application-specific templates. Call this before the first render so
// templates are parsed together with the engine ones.
func SetAppTemplatesFS(fsys fs.FS) {
	appTemplatesFS = fsys
}

// getGroupDefaults returns default metadata values for group plugin types.
// These are used when ui_meta_json is empty or missing fields.
func getGroupDefaults(elementKind string) map[string]interface{} {
	switch elementKind {
	case "group":
		return map[string]interface{}{
			"show_legend":  true,
			"border_style": "default",
		}
	case "accordion":
		return map[string]interface{}{
			"expanded_by_default": true,
			"show_header":         true,
		}
	case "card":
		return map[string]interface{}{
			"show_header": true,
			"header_bg":   "default",
		}
	case "tabs":
		return map[string]interface{}{
			"tab_position": "top",
			"active_tab":   0,
		}
	case "carousel":
		return map[string]interface{}{
			"show_controls":   true,
			"show_indicators": true,
			"interval":        5000,
			"fade":            false,
		}
	default:
		return map[string]interface{}{}
	}
}

// getFieldDefaults returns default metadata values for field plugin types.
// These are used when ui_meta_json is empty or missing fields.
func getFieldDefaults(uiKind string) map[string]interface{} {
	switch uiKind {
	case "text":
		return map[string]interface{}{
			"placeholder": "",
			"maxLength":   0,
			"pattern":     "",
			"inputMode":   "text",
		}
	case "textarea":
		return map[string]interface{}{
			"placeholder": "",
			"rows":        3,
			"maxLength":   0,
		}
	case "int":
		return map[string]interface{}{
			"min":         nil,
			"max":         nil,
			"step":        1,
			"placeholder": "",
		}
	case "decimal":
		return map[string]interface{}{
			"min":           nil,
			"max":           nil,
			"step":          "any",
			"decimalPlaces": 2,
			"placeholder":   "",
		}
	case "bool":
		return map[string]interface{}{
			"style": "select", // select, checkbox, switch
		}
	case "datetime":
		return map[string]interface{}{
			"includeTime": true,
			"minDate":     "",
			"maxDate":     "",
		}
	case "select":
		return map[string]interface{}{
			"options":    []interface{}{},
			"allowEmpty": true,
			"multiple":   false,
		}
	case "image":
		return map[string]interface{}{
			"alt_text": "",
		}
	case "video":
		return map[string]interface{}{
			"controls": true,
			"autoplay": false,
			"loop":     false,
			"muted":    false,
		}
	case "audio":
		return map[string]interface{}{
			"controls": true,
			"autoplay": false,
			"loop":     false,
			"muted":    false,
		}
	default:
		return map[string]interface{}{}
	}
}

// parseWithPatterns parses all matched files from fsys using the provided
// patterns. Patterns that match no files are ignored gracefully.
func parseWithPatterns(t *template.Template, fsys fs.FS, patterns ...string) (*template.Template, error) {
	if fsys == nil {
		return t, nil
	}

	var matches []string
	for _, pattern := range patterns {
		files, err := fs.Glob(fsys, pattern)
		if err != nil {
			return t, err
		}
		if len(files) > 0 {
			matches = append(matches, files...)
		}
	}

	if len(matches) == 0 {
		return t, nil
	}

	return t.ParseFS(fsys, matches...)
}

func loadTemplates() *template.Template {
	// Register small helper functions for templates
	funcMap := template.FuncMap{
		"split": strings.Split,
		"join":  strings.Join,
		"trim":  strings.TrimSpace,
		"first": func(s string) string {
			if len(s) == 0 {
				return ""
			}
			return string(s[0])
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		// seq generates a slice of integers from start to end (inclusive)
		"seq": func(start, end int) []int {
			if start > end {
				return nil
			}
			result := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
			return result
		},
		// deref dereferences a pointer to int64, returns 0 if nil
		"deref": func(p *int64) int64 {
			if p == nil {
				return 0
			}
			return *p
		},
		// add adds two integers
		"add": func(a, b int) int {
			return a + b
		},
		// mul multiplies two integers
		"mul": func(a, b int) int {
			return a * b
		},
		// dict creates a map from key-value pairs for passing to templates
		"dict": func(values ...interface{}) map[string]interface{} {
			if len(values)%2 != 0 {
				return nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					continue
				}
				dict[key] = values[i+1]
			}
			return dict
		},
		// parseGroupMeta parses ui_meta_json with defaults for group plugins
		"parseGroupMeta": func(jsonStr string, elementKind string) map[string]interface{} {
			defaults := getGroupDefaults(elementKind)
			if jsonStr == "" {
				return defaults
			}
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
				return defaults
			}
			// Merge: meta values override defaults
			for k, v := range meta {
				defaults[k] = v
			}
			return defaults
		},
		// parseFieldMeta parses ui_meta_json with defaults for field plugins
		"parseFieldMeta": func(jsonStr string, uiKind string) map[string]interface{} {
			defaults := getFieldDefaults(uiKind)
			if jsonStr == "" {
				return defaults
			}
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
				return defaults
			}
			// Merge: meta values override defaults
			for k, v := range meta {
				defaults[k] = v
			}
			return defaults
		},
	}

	base := template.New("").Funcs(funcMap)
	t, err := parseWithPatterns(base, filesystem,
		"*.go.tmpl",
		"partials/*.go.tmpl")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	// Parse extra filesystems from feature packages
	for _, fsys := range extraFS {
		t, err = parseWithPatterns(t, fsys,
			"templates/*.go.tmpl",
			"templates/partials/*.go.tmpl",
			"*.go.tmpl",
			"partials/*.go.tmpl")
		if err != nil {
			log.Fatalf("parse extra templates: %v", err)
		}
	}

	// Parse application templates (if configured)
	if appTemplatesFS != nil {
		t, err = parseWithPatterns(t, appTemplatesFS,
			"*.go.tmpl",
			"partials/*.go.tmpl",
			"templates/*.go.tmpl",
			"templates/partials/*.go.tmpl")
		if err != nil {
			log.Fatalf("parse app templates: %v", err)
		}
	}

	return t
}

func ensureTemplatesLoaded() {
	loadOnce.Do(func() {
		tpl = loadTemplates()
	})
}
