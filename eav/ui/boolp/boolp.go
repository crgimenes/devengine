// Package boolp registers the "bool" field plugin (BOOL primitive).
package boolp

import (
	"embed"
	"errors"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
	"github.com/crgimenes/devengine/templates"
)

//go:embed field_bool.go.tmpl
var templatesFS embed.FS

func init() {
	templates.RegisterFS(templatesFS)
	ui.Register("bool", func() ui.FieldUI { return Plugin{} })
}

type Plugin struct{}

func (Plugin) ID() string               { return "bool" }
func (Plugin) PrimitiveKinds() []string { return []string{"BOOL"} }
func (Plugin) HasPersistence() bool     { return true }
func (Plugin) SupportsReadOnly() bool   { return true }

func (Plugin) Defaults() map[string]any {
	return map[string]any{
		"style": "select", // select, checkbox, switch
	}
}

func (Plugin) ParseOptions(string) any { return nil }

func (Plugin) Parse(raw string, _ any) (any, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "false", "0", "off", "no":
		return false, nil
	case "true", "1", "on", "yes":
		return true, nil
	}
	return nil, errors.New("bool: invalid value")
}

func (Plugin) Validate(value, _ any) error {
	_, ok := value.(bool)
	if !ok {
		return errors.New("bool: value must be bool")
	}
	return nil
}
