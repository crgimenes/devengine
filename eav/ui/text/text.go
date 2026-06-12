// Package text registers the "text" field plugin (TEXT primitive).
package text

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/crgimenes/devengine/eav/ui"
	"github.com/crgimenes/devengine/templates"
)

//go:embed field_text.go.tmpl
var templatesFS embed.FS

func init() {
	templates.RegisterFS(templatesFS)
	ui.Register("text", func() ui.FieldUI { return Plugin{} })
}

type Options struct {
	Placeholder string `json:"placeholder,omitempty"`
	MinLength   int    `json:"min_length,omitempty"`
	MaxLength   int    `json:"max_length,omitempty"`
}

type Plugin struct{}

func (Plugin) ID() string               { return "text" }
func (Plugin) PrimitiveKinds() []string { return []string{"TEXT"} }
func (Plugin) HasPersistence() bool     { return true }
func (Plugin) SupportsReadOnly() bool   { return true }

func (Plugin) Defaults() map[string]any {
	return map[string]any{
		"placeholder": "",
		"maxLength":   0,
		"pattern":     "",
		"inputMode":   "text",
	}
}

func (Plugin) ParseOptions(raw string) any {
	var o Options
	if raw == "" {
		return o
	}
	_ = json.Unmarshal([]byte(raw), &o)
	return o
}

func (Plugin) Parse(raw string, _ any) (any, error) {
	return strings.TrimSpace(raw), nil
}

func (Plugin) Validate(value, opts any) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("text: value must be a string")
	}
	o, _ := opts.(Options)
	n := utf8.RuneCountInString(s)
	if o.MinLength > 0 && n < o.MinLength {
		return fmt.Errorf("text: too short (min %d)", o.MinLength)
	}
	if o.MaxLength > 0 && n > o.MaxLength {
		return fmt.Errorf("text: too long (max %d)", o.MaxLength)
	}
	return nil
}
