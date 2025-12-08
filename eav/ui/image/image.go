package image

import (
	"embed"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

type ImageUI struct{}

type Options struct{}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("image", func() ui.FieldUI {
		return &ImageUI{}
	}, templatesFS)
}

func (i *ImageUI) ID() string {
	return "image"
}

func (i *ImageUI) Label() string {
	return "Campo de imagem"
}

func (i *ImageUI) HasPersistence() bool {
	return true
}

func (i *ImageUI) SupportsReadOnly() bool {
	return true
}

func (i *ImageUI) IsGroupingField() bool {
	return false
}

func (i *ImageUI) RecommendedPrimitiveKind() string {
	return "TEXT"
}

func (i *ImageUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (i *ImageUI) decode(meta string) Options {
	opts := Options{}
	err := ui.DecodeOptions(meta, &opts)
	if err != nil {
		return Options{}
	}
	return opts
}

func (i *ImageUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	opts := i.decode(ctx.Field.UIMetaJSON)
	data := ui.TemplateData{Context: ctx, Options: opts}
	html, err := ui.RenderTemplateBuffer("eav_ui_image_options", data)
	if err != nil {
		return "", nil, err
	}
	return html, opts, nil
}

func (i *ImageUI) ParseOptions(_ url.Values) (json.RawMessage, error) {
	return ui.EncodeOptions(Options{})
}

func (i *ImageUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
	if ctx.Value == nil {
		return nil, nil
	}

	str, ok := ctx.Value.(string)
	if !ok {
		return ctx.Value, nil
	}

	trimmed := strings.TrimSpace(str)
	ctx.Value = trimmed
	if trimmed == "" {
		return nil, nil
	}
	return trimmed, nil
}

func (i *ImageUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
	if ctx.Value == nil {
		return ui.ValidationResult{OK: true}
	}

	str, ok := ctx.Value.(string)
	if !ok {
		return ui.ValidationResult{OK: false, Message: "Valor inválido"}
	}

	if strings.TrimSpace(str) == "" && ctx.Field.Required {
		return ui.ValidationResult{OK: false, Message: "Campo obrigatório"}
	}

	return ui.ValidationResult{OK: true}
}
