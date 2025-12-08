package video

import (
	"embed"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

type VideoUI struct{}

type Options struct{}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("video", func() ui.FieldUI {
		return &VideoUI{}
	}, templatesFS)
}

func (v *VideoUI) ID() string {
	return "video"
}

func (v *VideoUI) Label() string {
	return "Campo de vídeo"
}

func (v *VideoUI) HasPersistence() bool {
	return true
}

func (v *VideoUI) SupportsReadOnly() bool {
	return true
}

func (v *VideoUI) IsGroupingField() bool {
	return false
}

func (v *VideoUI) RecommendedPrimitiveKind() string {
	return "TEXT"
}

func (v *VideoUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (v *VideoUI) decode(meta string) Options {
	opts := Options{}
	err := ui.DecodeOptions(meta, &opts)
	if err != nil {
		return Options{}
	}
	return opts
}

func (v *VideoUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	opts := v.decode(ctx.Field.UIMetaJSON)
	data := ui.TemplateData{Context: ctx, Options: opts}

	html, err := ui.RenderTemplateBuffer("eav_ui_video_options", data)
	if err != nil {
		return "", nil, err
	}

	return html, opts, nil
}

func (v *VideoUI) ParseOptions(_ url.Values) (json.RawMessage, error) {
	return ui.EncodeOptions(Options{})
}

func (v *VideoUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
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

func (v *VideoUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
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
