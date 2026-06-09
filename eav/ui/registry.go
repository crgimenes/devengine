package ui

import (
	"slices"
	"sync"
)

type Factory func() FieldUI

var (
	mu  sync.RWMutex
	reg = map[string]Factory{}
)

// Register adds a plugin factory under the given id. Safe to call multiple
// times with the same id (last write wins) to keep tests forgiving.
func Register(id string, f Factory) {
	if id == "" {
		panic("ui: empty plugin id")
	}
	if f == nil {
		panic("ui: nil factory for plugin " + id)
	}
	mu.Lock()
	reg[id] = f
	mu.Unlock()
}

// Get returns a fresh plugin instance for id. The second result is false when
// id is not registered.
func Get(id string) (FieldUI, bool) {
	mu.RLock()
	f, ok := reg[id]
	mu.RUnlock()
	if !ok {
		return nil, false
	}
	return f(), true
}

// IDs returns the registered plugin ids in lexical order.
func IDs() []string {
	mu.RLock()
	out := make([]string, 0, len(reg))
	for id := range reg {
		out = append(out, id)
	}
	mu.RUnlock()
	slices.Sort(out)
	return out
}

// Reset clears the registry. For tests only.
func Reset() {
	mu.Lock()
	reg = map[string]Factory{}
	mu.Unlock()
}
