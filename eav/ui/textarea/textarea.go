// Package textarea registers the "textarea" field plugin (TEXT primitive).
package textarea

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/crgimenes/devengine/eav/ui"
)

func init() {
	ui.Register("textarea", func() ui.FieldUI { return Plugin{} })
}

type Options struct {
	Placeholder string `json:"placeholder,omitempty"`
	Rows        int    `json:"rows,omitempty"`
	MaxLength   int    `json:"max_length,omitempty"`
}

type Plugin struct{}

func (Plugin) ID() string               { return "textarea" }
func (Plugin) PrimitiveKinds() []string { return []string{"TEXT"} }
func (Plugin) HasPersistence() bool     { return true }
func (Plugin) SupportsReadOnly() bool   { return true }

func (Plugin) ParseOptions(raw string) any {
	var o Options
	if raw == "" {
		return o
	}
	_ = json.Unmarshal([]byte(raw), &o)
	return o
}

func (Plugin) Parse(raw string, _ any) (any, error) {
	return raw, nil
}

func (Plugin) Validate(value, opts any) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("textarea: value must be a string")
	}
	o, _ := opts.(Options)
	if o.MaxLength > 0 && utf8.RuneCountInString(s) > o.MaxLength {
		return fmt.Errorf("textarea: too long (max %d)", o.MaxLength)
	}
	return nil
}
