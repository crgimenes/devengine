package templates

import (
	"html/template"
	"io/fs"
	"log"
	"strings"
	"sync"
)

var (
	extraFS  []fs.FS
	loadOnce sync.Once
	tpl      *template.Template
)

// RegisterFS allows feature packages to contribute template file systems
// before the first render.
func RegisterFS(fsys fs.FS) {
	extraFS = append(extraFS, fsys)
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
	}

	base := template.New("").Funcs(funcMap)
	t, err := base.ParseFS(
		filesystem,
		"*.go.tmpl",
		"partials/*.go.tmpl",
	)
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	for _, fsys := range extraFS {
		t, err = t.ParseFS(fsys, "templates/*.go.tmpl")
		if err != nil {
			log.Fatalf("parse extra templates: %v", err)
		}
	}

	return t
}

func ensureTemplatesLoaded() {
	loadOnce.Do(func() {
		tpl = loadTemplates()
	})
}
