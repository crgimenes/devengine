package ui

import (
	"embed"
	"fmt"
	"sync"

	"github.com/crgimenes/devengine/templates"
)

type fieldUIEntry struct {
	factory    func() FieldUI
	templates  embed.FS
	identifier string
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]fieldUIEntry)
)

func RegisterFieldUI(id string, factory func() FieldUI, tplFS embed.FS) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if id == "" {
		panic("field UI id cannot be empty")
	}
	if _, exists := registry[id]; exists {
		panic(fmt.Sprintf("field UI %s already registered", id))
	}

	registry[id] = fieldUIEntry{factory: factory, templates: tplFS, identifier: id}
	templates.RegisterFS(tplFS)
}

func All() []FieldUI {
	registryMu.RLock()
	defer registryMu.RUnlock()

	out := make([]FieldUI, 0, len(registry))
	for _, entry := range registry {
		ui := entry.factory()
		out = append(out, ui)
	}
	return out
}

func Get(id string) (FieldUI, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	entry, ok := registry[id]
	if !ok {
		return nil, false
	}
	return entry.factory(), true
}

func TemplatesFS() []embed.FS {
	registryMu.RLock()
	defer registryMu.RUnlock()

	out := make([]embed.FS, 0, len(registry))
	for _, entry := range registry {
		out = append(out, entry.templates)
	}
	return out
}
