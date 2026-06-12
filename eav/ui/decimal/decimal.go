// Package decimal registers the "decimal" field plugin (REAL primitive).
//
// The id is "decimal" rather than "real" to align with the legacy partial
// (field_decimal.go.tmpl) and the implicit primitive->ui_kind mapping in
// forms_runtime.go.tmpl. The plugin operates on the REAL primitive type.
package decimal

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
	"github.com/crgimenes/devengine/templates"
)

//go:embed field_decimal.go.tmpl
var templatesFS embed.FS

func init() {
	templates.RegisterFS(templatesFS)
	ui.Register("decimal", func() ui.FieldUI { return Plugin{} })
}

type Options struct {
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
	Step float64  `json:"step,omitempty"`
}

type Plugin struct{}

func (Plugin) ID() string               { return "decimal" }
func (Plugin) PrimitiveKinds() []string { return []string{"REAL"} }
func (Plugin) HasPersistence() bool     { return true }
func (Plugin) SupportsReadOnly() bool   { return true }

func (Plugin) Defaults() map[string]any {
	return map[string]any{
		"min":           nil,
		"max":           nil,
		"step":          "any",
		"decimalPlaces": 2,
		"placeholder":   "",
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
	s := strings.TrimSpace(raw)
	if s == "" {
		return float64(0), nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("decimal: invalid number %q", s)
	}
	return v, nil
}

func (Plugin) Validate(value, opts any) error {
	v, ok := value.(float64)
	if !ok {
		return errors.New("decimal: value must be float64")
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return errors.New("decimal: NaN or Inf not allowed")
	}
	o, _ := opts.(Options)
	if o.Min != nil && v < *o.Min {
		return fmt.Errorf("decimal: below minimum %g", *o.Min)
	}
	if o.Max != nil && v > *o.Max {
		return fmt.Errorf("decimal: above maximum %g", *o.Max)
	}
	return nil
}
