package groupaccordion

import (
	"embed"
	"encoding/json"
	"net/url"

	"github.com/crgimenes/devengine/eav/ui"
)

type AccordionUI struct{}

type Options struct {
	StartOpen         bool   `json:"start_open,omitempty"`
	AccordionGroupKey string `json:"accordion_group_key,omitempty"`
}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("group_accordion", func() ui.FieldUI {
		return &AccordionUI{}
	}, templatesFS)
}

func (a *AccordionUI) ID() string {
	return "group_accordion"
}

func (a *AccordionUI) Label() string {
	return "Accordion"
}

func (a *AccordionUI) HasPersistence() bool {
	return false
}

func (a *AccordionUI) SupportsReadOnly() bool {
	return false
}

func (a *AccordionUI) IsGroupingField() bool {
	return true
}

func (a *AccordionUI) RecommendedPrimitiveKind() string {
	return "-"
}

func (a *AccordionUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (a *AccordionUI) decode(meta string) Options {
	opts := Options{}
	err := ui.DecodeOptions(meta, &opts)
	if err != nil {
		return Options{}
	}
	return opts
}

func (a *AccordionUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	opts := a.decode(ctx.Field.UIMetaJSON)
	data := ui.TemplateData{Context: ctx, Options: opts}
	buf, err := ui.RenderTemplateBuffer("eav_ui_group_accordion_options", data)
	if err != nil {
		return "", nil, err
	}
	return buf, opts, nil
}

func (a *AccordionUI) ParseOptions(form url.Values) (json.RawMessage, error) {
	startOpen := form.Get("group_accordion_start_open") == "on"

	opts := Options{StartOpen: startOpen}

	return ui.EncodeOptions(opts)
}

func (a *AccordionUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
	return nil, nil
}

func (a *AccordionUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
	return ui.ValidationResult{OK: true}
}
