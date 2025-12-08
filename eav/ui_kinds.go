package eav

import (
	"sort"
	"strings"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/eav/ui"
	"github.com/crgimenes/devengine/eav/uiplugins"
)

type uiKindOption struct {
	Value  string
	Label  string
	Detail string
}

const defaultUIKind = "text"

var (
	fieldUIKindOptions       = []uiKindOption{}
	allowedUIKinds           = map[string]struct{}{}
	suppressedRuntimeUIKinds = map[string]struct{}{}
)

func init() {
	uiplugins.Init()
	refreshUIKinds()
}

func uiKindSelectOptions() []uiKindOption {
	return fieldUIKindOptions
}

func isValidUIKind(value string) bool {
	if value == "" {
		return false
	}
	_, ok := allowedUIKinds[value]
	return ok
}

func shouldSuppressRuntimeField(f db.EAVField) bool {
	if f.IsUI {
		return false
	}
	if !f.Visible {
		return true
	}
	_, suppressed := suppressedRuntimeUIKinds[strings.ToLower(f.UIKind)]
	return suppressed
}

func refreshUIKinds() {
	uis := ui.All()
	fieldUIKindOptions = fieldUIKindOptions[:0]
	allowedUIKinds = map[string]struct{}{}

	sort.Slice(uis, func(i, j int) bool {
		return uis[i].Label() < uis[j].Label()
	})

	for _, item := range uis {
		allowedUIKinds[item.ID()] = struct{}{}
		fieldUIKindOptions = append(fieldUIKindOptions, uiKindOption{
			Value:  item.ID(),
			Label:  item.Label(),
			Detail: "",
		})
	}
}
