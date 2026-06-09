// Package intp registers the "int" field plugin (INT primitive).
package intp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

func init() {
	ui.Register("int", func() ui.FieldUI { return Plugin{} })
}

type Options struct {
	Min  *int64 `json:"min,omitempty"`
	Max  *int64 `json:"max,omitempty"`
	Step int64  `json:"step,omitempty"`
}

type Plugin struct{}

func (Plugin) ID() string               { return "int" }
func (Plugin) PrimitiveKinds() []string { return []string{"INT"} }
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
	s := strings.TrimSpace(raw)
	if s == "" {
		return int64(0), nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("int: invalid number %q", s)
	}
	return v, nil
}

func (Plugin) Validate(value, opts any) error {
	v, ok := value.(int64)
	if !ok {
		return errors.New("int: value must be int64")
	}
	o, _ := opts.(Options)
	if o.Min != nil && v < *o.Min {
		return fmt.Errorf("int: below minimum %d", *o.Min)
	}
	if o.Max != nil && v > *o.Max {
		return fmt.Errorf("int: above maximum %d", *o.Max)
	}
	return nil
}
