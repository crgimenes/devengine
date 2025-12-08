package integer

import (
	"embed"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

type IntegerUI struct{}

type Options struct {
	Min *int64 `json:"min,omitempty"`
	Max *int64 `json:"max,omitempty"`
}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("integer", func() ui.FieldUI {
		return &IntegerUI{}
	}, templatesFS)
}

func (i *IntegerUI) ID() string {
	return "integer"
}

func (i *IntegerUI) Label() string {
	return "Número inteiro"
}

func (i *IntegerUI) HasPersistence() bool {
	return true
}

func (i *IntegerUI) SupportsReadOnly() bool {
	return true
}

func (i *IntegerUI) IsGroupingField() bool {
	return false
}

func (i *IntegerUI) RecommendedPrimitiveKind() string {
	return "INT"
}

func (i *IntegerUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (i *IntegerUI) decode(meta string) Options {
	opts := Options{}
	err := ui.DecodeOptions(meta, &opts)
	if err != nil {
		return Options{}
	}
	return opts
}

func (i *IntegerUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	opts := i.decode(ctx.Field.UIMetaJSON)
	data := ui.TemplateData{Context: ctx, Options: opts}
	html, err := ui.RenderTemplateBuffer("eav_ui_integer_options", data)
	if err != nil {
		return "", nil, err
	}
	return html, opts, nil
}

func (i *IntegerUI) ParseOptions(form url.Values) (json.RawMessage, error) {
	opts := Options{}

	rawMin := strings.TrimSpace(form.Get("integer_min"))
	if rawMin != "" {
		parsed, err := strconv.ParseInt(rawMin, 10, 64)
		if err != nil {
			return nil, err
		}
		opts.Min = &parsed
	}

	rawMax := strings.TrimSpace(form.Get("integer_max"))
	if rawMax != "" {
		parsed, err := strconv.ParseInt(rawMax, 10, 64)
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

func (i *IntegerUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
	return ctx.Value, nil
}

func (i *IntegerUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
	opts := i.decode(ctx.Field.UIMetaJSON)
	val, ok := ctx.Value.(int64)
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
