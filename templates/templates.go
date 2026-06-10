package templates

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"maps"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/crgimenes/devengine/log"
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

// ReferenceChoice is one entry in a reference field's choice list.
type ReferenceChoice struct {
	Value string
	Label string
}

// referenceChoicesFn is the lookup hook installed by the reference plugin.
// Templates expose it as `eavReferenceChoices`. When unset the function
// returns an empty list so partials still render.
var referenceChoicesFn func(entity, displayAttr string) []ReferenceChoice

// RegisterReferenceChoicesProvider installs the lookup used to populate
// reference field selects.
func RegisterReferenceChoicesProvider(fn func(entity, displayAttr string) []ReferenceChoice) {
	referenceChoicesFn = fn
}

// SubformRecord is one row in a subform listing: the related record's
// reference_id (used for the edit link) and the formatted label.
type SubformRecord struct {
	ReferenceID string
	Label       string
}

var subformRecordsFn func(targetEntity, targetAttr, parentRef, displayAttr string) []SubformRecord

// RegisterSubformRecordsProvider installs the lookup used to populate
// subform listings.
func RegisterSubformRecordsProvider(fn func(targetEntity, targetAttr, parentRef, displayAttr string) []SubformRecord) {
	subformRecordsFn = fn
}

// SetAppTemplatesFS configures an optional filesystem containing
// application-specific templates. Call this before the first render so
// templates are parsed together with the engine ones.
func SetAppTemplatesFS(fsys fs.FS) {
	appTemplatesFS = fsys
}

// getGroupDefaults returns default metadata values for group plugin types.
// These are used when ui_meta_json is empty or missing fields.
func getGroupDefaults(elementKind string) map[string]any {
	switch elementKind {
	case "group":
		return map[string]any{
			"show_legend":  true,
			"border_style": "default",
		}
	case "accordion":
		return map[string]any{
			"expanded_by_default": true,
			"show_header":         true,
		}
	case "card":
		return map[string]any{
			"show_header": true,
			"header_bg":   "default",
		}
	case "tabs":
		return map[string]any{
			"tab_position": "top",
			"active_tab":   0,
		}
	case "carousel":
		return map[string]any{
			"show_controls":   true,
			"show_indicators": true,
			"interval":        5000,
			"fade":            false,
		}
	default:
		return map[string]any{}
	}
}

// getFieldDefaults returns default metadata values for field plugin types.
// These are used when ui_meta_json is empty or missing fields.
func getFieldDefaults(uiKind string) map[string]any {
	switch uiKind {
	case "text":
		return map[string]any{
			"placeholder": "",
			"maxLength":   0,
			"pattern":     "",
			"inputMode":   "text",
		}
	case "textarea":
		return map[string]any{
			"placeholder": "",
			"rows":        3,
			"maxLength":   0,
		}
	case "int":
		return map[string]any{
			"min":         nil,
			"max":         nil,
			"step":        1,
			"placeholder": "",
		}
	case "decimal":
		return map[string]any{
			"min":           nil,
			"max":           nil,
			"step":          "any",
			"decimalPlaces": 2,
			"placeholder":   "",
		}
	case "bool":
		return map[string]any{
			"style": "select", // select, checkbox, switch
		}
	case "datetime":
		return map[string]any{
			"includeTime": true,
			"minDate":     "",
			"maxDate":     "",
		}
	case "select":
		return map[string]any{
			"options":    []any{},
			"allowEmpty": true,
			"multiple":   false,
		}
	case "image":
		return map[string]any{
			"alt_text": "",
		}
	case "video":
		return map[string]any{
			"controls": true,
			"autoplay": false,
			"loop":     false,
			"muted":    false,
		}
	case "audio":
		return map[string]any{
			"controls": true,
			"autoplay": false,
			"loop":     false,
			"muted":    false,
		}
	default:
		return map[string]any{}
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
		// safeHTML is the deliberate opt-in for embedding trusted markup;
		// callers own the responsibility of never passing user input.
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s) // #nosec G203 -- explicit trusted-content escape hatch
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
		// htmlDatetime normalizes a stored datetime into the format HTML5
		// datetime-local / date inputs accept (no timezone suffix).
		"htmlDatetime": htmlDatetime,
		// fmtDatetime formats a stored datetime for human display.
		"fmtDatetime": fmtDatetime,
		// toJSON marshals a value to a JSON string for embedding in attributes.
		"toJSON": func(v any) string {
			b, err := json.Marshal(v)
			if err != nil {
				return "null"
			}
			return string(b)
		},
		// dict creates a map from key-value pairs for passing to templates
		"dict": func(values ...any) map[string]any {
			if len(values)%2 != 0 {
				return nil
			}
			dict := make(map[string]any, len(values)/2)
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
		"parseGroupMeta": func(jsonStr string, elementKind string) map[string]any {
			defaults := getGroupDefaults(elementKind)
			if jsonStr == "" {
				return defaults
			}
			var meta map[string]any
			if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
				return defaults
			}
			// Merge: meta values override defaults
			maps.Copy(defaults, meta)
			return defaults
		},
		"renderField": renderField,
		// eavReferenceChoices returns the list of choices for a reference
		// field. Accepts any-typed args so it composes cleanly with
		// `(index .Meta "...")` from the partial.
		"eavReferenceChoices": func(entity, displayAttr any) []ReferenceChoice {
			if referenceChoicesFn == nil {
				return nil
			}
			e, _ := entity.(string)
			d, _ := displayAttr.(string)
			if e == "" {
				return nil
			}
			return referenceChoicesFn(e, d)
		},
		// eavSubformRecords returns the related records for a subform field.
		"eavSubformRecords": func(targetEntity, targetAttr, parentRef, displayAttr any) []SubformRecord {
			if subformRecordsFn == nil {
				return nil
			}
			te, _ := targetEntity.(string)
			ta, _ := targetAttr.(string)
			pr, _ := parentRef.(string)
			da, _ := displayAttr.(string)
			if te == "" || ta == "" || pr == "" {
				return nil
			}
			return subformRecordsFn(te, ta, pr, da)
		},
		// parseFieldMeta parses ui_meta_json with defaults for field plugins
		"parseFieldMeta": func(jsonStr string, uiKind string) map[string]any {
			defaults := getFieldDefaults(uiKind)
			if jsonStr == "" {
				return defaults
			}
			var meta map[string]any
			if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
				return defaults
			}
			// Merge: meta values override defaults
			maps.Copy(defaults, meta)
			return defaults
		},
		// getMenuItems safely extracts MenuItems field from any struct using reflection.
		// Returns nil if the field doesn't exist or is nil/empty.
		"getMenuItems": func(data any) any {
			if data == nil {
				return nil
			}
			v := reflect.ValueOf(data)
			if v.Kind() == reflect.Pointer {
				v = v.Elem()
			}
			if v.Kind() != reflect.Struct {
				return nil
			}
			f := v.FieldByName("MenuItems")
			if !f.IsValid() {
				return nil
			}
			// Check if the field is a slice or array and has elements
			switch f.Kind() {
			case reflect.Slice, reflect.Array:
				if f.IsNil() || f.Len() == 0 {
					return nil
				}
			case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Chan, reflect.Func:
				if f.IsNil() {
					return nil
				}
			}
			return f.Interface()
		},
		// getMenuMachineName safely extracts MenuMachineName field from any struct using reflection.
		// Returns empty string if the field doesn't exist.
		"getMenuMachineName": func(data any) string {
			if data == nil {
				return ""
			}
			v := reflect.ValueOf(data)
			if v.Kind() == reflect.Pointer {
				v = v.Elem()
			}
			if v.Kind() != reflect.Struct {
				return ""
			}
			f := v.FieldByName("MenuMachineName")
			if !f.IsValid() {
				return ""
			}
			if f.Kind() != reflect.String {
				return ""
			}
			return f.String()
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

// executeBuffered renders into a buffer and only writes to w on success.
// Writing straight to the ResponseWriter means an error halfway through
// execution ships a partial page with a 200 status, and the handler's
// http.Error afterwards becomes a no-op (superfluous WriteHeader).
func executeBuffered(w io.Writer, templateName string, data any) error {
	var buf bytes.Buffer
	err := tpl.ExecuteTemplate(&buf, templateName, data)
	if err != nil {
		return err
	}
	_, err = w.Write(buf.Bytes())
	return err
}

// fmtDatetime formats a stored datetime for human display (dd/mm/yyyy HH:MM).
// Non-strings and values that do not fully parse as a datetime are returned
// unchanged, so it is safe to apply generically to mixed value columns.
func fmtDatetime(value any) string {
	if t, ok := value.(time.Time); ok {
		return t.Format("02/01/2006 15:04")
	}
	s, ok := value.(string)
	if !ok {
		return fmt.Sprint(value)
	}
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	} {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t.Format("02/01/2006 15:04")
		}
	}
	return s
}

// htmlDatetime normalizes a stored datetime into the format HTML5
// datetime-local / date inputs accept (no timezone suffix). The SQLite driver
// reads DATETIME columns back as RFC3339 with a "Z", which those inputs reject,
// so we reparse and reformat. includeTime defaults to true; pass false for a
// date-only field. Returns "" when the value is empty or unparseable.
func htmlDatetime(value any, includeTime any) string {
	s, _ := value.(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var t time.Time
	parsed := false
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if tt, err := time.Parse(layout, s); err == nil {
			t = tt
			parsed = true
			break
		}
	}
	if !parsed {
		return ""
	}
	if inc, ok := includeTime.(bool); ok && !inc {
		return t.Format("2006-01-02")
	}
	return t.Format("2006-01-02T15:04")
}

// renderField executes the partial named `name` against data. Falls back to
// field_text when `name` is unknown so a form with an unregistered ui_kind
// still renders something usable.
func renderField(name string, data any) (template.HTML, error) {
	if tpl == nil {
		return "", errors.New("templates not loaded")
	}
	t := tpl.Lookup(name)
	if t == nil {
		t = tpl.Lookup("field_text")
	}
	if t == nil {
		return "", fmt.Errorf("template %q not found and field_text fallback missing", name)
	}
	var buf strings.Builder
	err := t.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil // #nosec G203 -- buf was produced by html/template which already escaped untrusted values
}
