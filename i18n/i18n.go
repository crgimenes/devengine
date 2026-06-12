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
	"sort"
	"strings"
	"sync"
)

const DefaultLocale = "en-US"

var (
	mu        sync.RWMutex
	locale    = DefaultLocale
	dicts     = map[string]map[string]string{}
	overrides = map[string]map[string]string{}
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

// T translates msg into the process-default locale. Prefer TL with the
// request locale in HTTP handlers; T remains for boot/CLI messages and as
// the fallback when no request is in sight.
func T(msg string, args ...any) string {
	return TL(Locale(), msg, args...)
}

// TL translates msg into the given locale and applies fmt args when present.
// The translation is looked up BEFORE formatting, so dictionary entries keep
// the original format verbs. Unknown locales and missing entries fall back
// to the US English source text.
func TL(loc, msg string, args ...any) string {
	mu.RLock()
	t, ok := overrides[loc][msg]
	if !ok {
		t, ok = dicts[loc][msg]
	}
	mu.RUnlock()
	if ok {
		msg = t
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// SetOverride layers a user adjustment on top of the built-in dictionary of
// a locale. It wins over Register entries and never mutates them, so
// DeleteOverride restores the shipped translation.
func SetOverride(loc, key, text string) {
	mu.Lock()
	defer mu.Unlock()
	o := overrides[loc]
	if o == nil {
		o = make(map[string]string)
		overrides[loc] = o
	}
	o[key] = text
}

// DeleteOverride removes a user adjustment, falling back to the built-in
// dictionary entry (or the English source when there is none).
func DeleteOverride(loc, key string) {
	mu.Lock()
	defer mu.Unlock()
	delete(overrides[loc], key)
}

// Override returns the user adjustment for a key, if any.
func Override(loc, key string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	t, ok := overrides[loc][key]
	return t, ok
}

// Keys returns every known message key (the union of all dictionaries and
// overrides), sorted. Keys ARE the US English source strings.
func Keys() []string {
	mu.RLock()
	set := map[string]bool{}
	for _, d := range dicts {
		for k := range d {
			set[k] = true
		}
	}
	for _, o := range overrides {
		for k := range o {
			set[k] = true
		}
	}
	mu.RUnlock()
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Locales lists the selectable locales: the default plus every registered
// dictionary, sorted.
func Locales() []string {
	mu.RLock()
	set := map[string]bool{DefaultLocale: true}
	for loc := range dicts {
		set[loc] = true
	}
	for loc := range overrides {
		set[loc] = true
	}
	mu.RUnlock()
	out := make([]string, 0, len(set))
	for loc := range set {
		out = append(out, loc)
	}
	sort.Strings(out)
	return out
}

// Known reports whether loc is the default locale or has a dictionary.
func Known(loc string) bool {
	if loc == DefaultLocale {
		return true
	}
	mu.RLock()
	_, ok := dicts[loc]
	if !ok {
		_, ok = overrides[loc]
	}
	mu.RUnlock()
	return ok
}

// MatchHeader picks the best known locale from an Accept-Language header,
// honoring its order (quality weights are ignored: browsers already sort).
// Falls back to prefix matching ("pt" picks pt-BR) and returns "" when
// nothing matches.
func MatchHeader(header string) string {
	for tag := range strings.SplitSeq(header, ",") {
		tag = strings.TrimSpace(tag)
		if i := strings.IndexByte(tag, ';'); i >= 0 {
			tag = strings.TrimSpace(tag[:i])
		}
		if tag == "" || tag == "*" {
			continue
		}
		for _, known := range Locales() {
			if strings.EqualFold(tag, known) {
				return known
			}
		}
		lang, _, _ := strings.Cut(tag, "-")
		for _, known := range Locales() {
			kl, _, _ := strings.Cut(known, "-")
			if strings.EqualFold(lang, kl) {
				return known
			}
		}
	}
	return ""
}

// content holds user-content translations: locale → refID+"\x00"+field →
// text. Labels of forms, elements, attributes and menu items live in the
// database in one language; these entries override them per locale at
// render time (runtime screens only — authoring screens show the original).
var content = map[string]map[string]string{}

func contentKey(refID, field string) string { return refID + "\x00" + field }

// SetContent stores a user-content translation in memory.
func SetContent(loc, refID, field, text string) {
	mu.Lock()
	defer mu.Unlock()
	c := content[loc]
	if c == nil {
		c = make(map[string]string)
		content[loc] = c
	}
	c[contentKey(refID, field)] = text
}

// DeleteContent removes a user-content translation.
func DeleteContent(loc, refID, field string) {
	mu.Lock()
	defer mu.Unlock()
	delete(content[loc], contentKey(refID, field))
}

// Content returns the translation for one field of one object, if any.
func Content(loc, refID, field string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	t, ok := content[loc][contentKey(refID, field)]
	return t, ok
}

// ContentOr returns the translated field text, falling back to the original.
func ContentOr(loc, refID, field, original string) string {
	t, ok := Content(loc, refID, field)
	if !ok || t == "" {
		return original
	}
	return t
}
