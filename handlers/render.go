package handlers

import (
	"net/http"

	"github.com/crgimenes/devengine/log"
)

// render executes a template and answers a clean 500 on failure. Safe because
// ExecuteTemplate buffers: nothing reaches the wire when execution errors.
func (h *Handlers) render(w http.ResponseWriter, name string, data any) {
	err := h.templates(w, name, data)
	if err != nil {
		log.Printf("template error in %s: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
