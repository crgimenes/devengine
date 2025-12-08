package audio

import (
	"embed"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

type AudioUI struct{}

type Options struct{}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("audio", func() ui.FieldUI {
		return &AudioUI{}
	}, templatesFS)
}

func (a *AudioUI) ID() string {
	return "audio"
}

func (a *AudioUI) Label() string {
	return "Campo de áudio"
}

func (a *AudioUI) HasPersistence() bool {
	return true
}

func (a *AudioUI) SupportsReadOnly() bool {
	return true
}

func (a *AudioUI) IsGroupingField() bool {
	return false
}

func (a *AudioUI) RecommendedPrimitiveKind() string {
	return "TEXT"
}

func (a *AudioUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (a *AudioUI) decode(meta string) Options {
	opts := Options{}
	err := ui.DecodeOptions(meta, &opts)
	if err != nil {
		return Options{}
	}
	return opts
}

func (a *AudioUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	opts := a.decode(ctx.Field.UIMetaJSON)
	data := ui.TemplateData{Context: ctx, Options: opts}

	html, err := ui.RenderTemplateBuffer("eav_ui_audio_options", data)
	if err != nil {
		return "", nil, err
	}

	return html, opts, nil
}

func (a *AudioUI) ParseOptions(_ url.Values) (json.RawMessage, error) {
	return ui.EncodeOptions(Options{})
}

func (a *AudioUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
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

func (a *AudioUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
	if ctx.Value == nil {
		if ctx.Field.Required {
			return ui.ValidationResult{OK: false, Message: "Campo obrigatório"}
		}
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
