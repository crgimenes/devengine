// Package i18n translates user-facing strings. It is gettext-shaped: the US
// English source text is the key, so the default locale (en-US) needs no
// dictionary at all and a missing entry falls back to English instead of
// breaking the page. Dictionaries are plain string maps registered per
// locale; the engine ships pt-BR built in and applications may extend it or
// add new locales.
package i18n

import (
	"fmt"
	"maps"
	"sync"
)

const DefaultLocale = "en-US"

var (
	mu     sync.RWMutex
	locale = DefaultLocale
	dicts  = map[string]map[string]string{}
)

// SetLocale switches the process-wide UI language. Unknown locales behave as
// en-US because lookups just miss.
func SetLocale(l string) {
	if l == "" {
		l = DefaultLocale
	}
	mu.Lock()
	locale = l
	mu.Unlock()
}

// Locale returns the active locale.
func Locale() string {
	mu.RLock()
	defer mu.RUnlock()
	return locale
}

// Register merges entries into the dictionary of a locale. Applications call
// it for their own strings or to override engine translations.
func Register(loc string, entries map[string]string) {
	mu.Lock()
	defer mu.Unlock()
	d := dicts[loc]
	if d == nil {
		d = make(map[string]string, len(entries))
		dicts[loc] = d
	}
	maps.Copy(d, entries)
}

// T translates msg into the active locale and applies fmt args when present.
// The translation is looked up BEFORE formatting, so dictionary entries keep
// the original format verbs.
func T(msg string, args ...any) string {
	mu.RLock()
	d := dicts[locale]
	mu.RUnlock()
	if t, ok := d[msg]; ok {
		msg = t
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}
