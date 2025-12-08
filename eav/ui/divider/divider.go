package divider

import (
	"embed"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

type DividerUI struct{}

type Options struct {
	Class string `json:"class,omitempty"`
}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("divider", func() ui.FieldUI {
		return &DividerUI{}
	}, templatesFS)
}

func (d *DividerUI) ID() string {
	return "divider"
}

func (d *DividerUI) Label() string {
	return "Divisor (linha horizontal)"
}

func (d *DividerUI) HasPersistence() bool {
	return false
}

func (d *DividerUI) SupportsReadOnly() bool {
	return false
}

func (d *DividerUI) IsGroupingField() bool {
	return false
}

func (d *DividerUI) RecommendedPrimitiveKind() string {
	return "TEXT"
}

func (d *DividerUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (d *DividerUI) decode(meta string) Options {
	opts := Options{}
	err := ui.DecodeOptions(meta, &opts)
	if err != nil {
		return Options{}
	}
	return opts
}

func (d *DividerUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	opts := d.decode(ctx.Field.UIMetaJSON)
	data := ui.TemplateData{Context: ctx, Options: opts}
	buf, err := ui.RenderTemplateBuffer("eav_ui_divider_options", data)
	if err != nil {
		return "", nil, err
	}
	return buf, opts, nil
}

func (d *DividerUI) ParseOptions(form url.Values) (json.RawMessage, error) {
	className := strings.TrimSpace(form.Get("divider_class"))
	if className == "" {
		return ui.EncodeOptions(Options{})
	}
	return ui.EncodeOptions(Options{Class: className})
}

func (d *DividerUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
	return nil, nil
}

func (d *DividerUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
	return ui.ValidationResult{OK: true}
}
