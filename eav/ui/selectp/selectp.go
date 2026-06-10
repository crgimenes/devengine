// Package selectp registers the "select" field plugin (TEXT or INT primitive).
//
// Options.Kind selects which primitive the plugin parses and stores into.
// Options.Options is the static list of choices the admin defines. A future
// version will accept a Filo expression to populate the list dynamically.
package selectp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

func init() {
	ui.Register("select", func() ui.FieldUI { return Plugin{} })
}

type Choice struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

type Options struct {
	Kind       string   `json:"kind,omitempty"` // "text" (default) or "int"
	Options    []Choice `json:"options,omitempty"`
	AllowEmpty bool     `json:"allow_empty,omitempty"`
}

type Plugin struct{}

func (Plugin) ID() string               { return "select" }
func (Plugin) PrimitiveKinds() []string { return []string{"TEXT", "INT"} }
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

func (Plugin) Parse(raw string, opts any) (any, error) {
	o, _ := opts.(Options)
	s := strings.TrimSpace(raw)
	if o.Kind == "int" {
		if s == "" {
			return int64(0), nil
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("select: invalid int %q", s)
		}
		return v, nil
	}
	return s, nil
}

func (Plugin) Validate(value, opts any) error {
	o, _ := opts.(Options)
	if len(o.Options) == 0 {
		return nil
	}

	var asString string
	switch v := value.(type) {
	case string:
		asString = v
	case int64:
		asString = strconv.FormatInt(v, 10)
	default:
		return errors.New("select: unsupported value type")
	}

	if asString == "" {
		if o.AllowEmpty {
			return nil
		}
		return errors.New("select: empty value not allowed")
	}

	for _, c := range o.Options {
		if c.Value == asString {
			return nil
		}
	}
	return fmt.Errorf("select: %q is not in the option list", asString)
}
