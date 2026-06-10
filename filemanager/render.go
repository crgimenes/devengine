package filemanager

import (
	"net/http"

	"github.com/crgimenes/devengine/log"
	"github.com/crgimenes/devengine/templates"
)

// renderOr500 executes a template and answers a clean 500 on failure. Safe
// because ExecuteTemplate buffers: nothing reaches the wire when it errors.
func renderOr500(w http.ResponseWriter, name string, data any) {
	err := templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("template error in %s: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
