package decimal

import (
	"embed"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

type DecimalUI struct{}

type Options struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("decimal", func() ui.FieldUI {
		return &DecimalUI{}
	}, templatesFS)
}

func (d *DecimalUI) ID() string {
	return "decimal"
}

func (d *DecimalUI) Label() string {
	return "Número decimal"
}

func (d *DecimalUI) HasPersistence() bool {
	return true
}

func (d *DecimalUI) SupportsReadOnly() bool {
	return true
}

func (d *DecimalUI) IsGroupingField() bool {
	return false
}

func (d *DecimalUI) RecommendedPrimitiveKind() string {
	return "FLOAT"
}

func (d *DecimalUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (d *DecimalUI) decode(meta string) Options {
	opts := Options{}
	err := ui.DecodeOptions(meta, &opts)
	if err != nil {
		return Options{}
	}
	return opts
}

func (d *DecimalUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	opts := d.decode(ctx.Field.UIMetaJSON)
	data := ui.TemplateData{Context: ctx, Options: opts}
	html, err := ui.RenderTemplateBuffer("eav_ui_decimal_options", data)
	if err != nil {
		return "", nil, err
	}
	return html, opts, nil
}

func (d *DecimalUI) ParseOptions(form url.Values) (json.RawMessage, error) {
	opts := Options{}

	rawMin := strings.TrimSpace(form.Get("decimal_min"))
	if rawMin != "" {
		parsed, err := strconv.ParseFloat(rawMin, 64)
		if err != nil {
			return nil, err
		}
		opts.Min = &parsed
	}

	rawMax := strings.TrimSpace(form.Get("decimal_max"))
	if rawMax != "" {
		parsed, err := strconv.ParseFloat(rawMax, 64)
		if err != nil {
			return nil, err
		}
		opts.Max = &parsed
	}

	if opts.Min != nil && opts.Max != nil && *opts.Min > *opts.Max {
		return nil, strconv.ErrRange
	}

	return ui.EncodeOptions(opts)
}

func (d *DecimalUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
	return ctx.Value, nil
}

func (d *DecimalUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
	opts := d.decode(ctx.Field.UIMetaJSON)
	val, ok := ctx.Value.(float64)
	if !ok {
		return ui.ValidationResult{OK: true}
	}
	if opts.Min != nil && val < *opts.Min {
		return ui.ValidationResult{OK: false, Message: "Abaixo do mínimo"}
	}
	if opts.Max != nil && val > *opts.Max {
		return ui.ValidationResult{OK: false, Message: "Acima do máximo"}
	}
	return ui.ValidationResult{OK: true}
}
