//go:build dev

package templates

import (
	"io"
	"os"
)

var (
	filesystem = os.DirFS("./templates")
)

func ExecuteTemplate(w io.Writer, templateName string, data any) error {
	tpl = loadTemplates()
	return tpl.ExecuteTemplate(w, templateName, data)
}
