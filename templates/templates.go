package templates

import (
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
