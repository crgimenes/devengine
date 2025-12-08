package boolean

import (
	"embed"
	"encoding/json"
	"net/url"

	"github.com/crgimenes/devengine/eav/ui"
)

type BooleanUI struct{}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("boolean", func() ui.FieldUI {
		return &BooleanUI{}
	}, templatesFS)
}

func (b *BooleanUI) ID() string {
	return "boolean"
}

func (b *BooleanUI) Label() string {
	return "Booleano"
}

func (b *BooleanUI) HasPersistence() bool {
	return true
}

func (b *BooleanUI) SupportsReadOnly() bool {
	return true
}

func (b *BooleanUI) IsGroupingField() bool {
	return false
}

func (b *BooleanUI) RecommendedPrimitiveKind() string {
	return "BOOL"
}

func (b *BooleanUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (b *BooleanUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	data := ui.TemplateData{Context: ctx}
	html, err := ui.RenderTemplateBuffer("eav_ui_boolean_options", data)
	if err != nil {
		return "", nil, err
	}
	return html, nil, nil
}

func (b *BooleanUI) ParseOptions(form url.Values) (json.RawMessage, error) {
	return ui.EncodeOptions(struct{}{})
}

func (b *BooleanUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
	return ctx.Value, nil
}

func (b *BooleanUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
	return ui.ValidationResult{OK: true}
}
