package text

import (
	"embed"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

type TextUI struct{}

type Options struct {
	MaxLength int `json:"max_length,omitempty"`
}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("text", func() ui.FieldUI {
		return &TextUI{}
	}, templatesFS)
}

func (t *TextUI) ID() string {
	return "text"
}

func (t *TextUI) Label() string {
	return "Campo de texto"
}

func (t *TextUI) HasPersistence() bool {
	return true
}

func (t *TextUI) SupportsReadOnly() bool {
	return true
}

func (t *TextUI) IsGroupingField() bool {
	return false
}

func (t *TextUI) RecommendedPrimitiveKind() string {
	return "TEXT"
}

func (t *TextUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (t *TextUI) decode(meta string) Options {
	opts := Options{}
	err := ui.DecodeOptions(meta, &opts)
	if err != nil {
		return Options{}
	}
	return opts
}

func (t *TextUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	opts := t.decode(ctx.Field.UIMetaJSON)
	data := ui.TemplateData{
		Context: ctx,
		Options: opts,
	}
	buf, err := ui.RenderTemplateBuffer("eav_ui_text_options", data)
	if err != nil {
		return "", nil, err
	}
	return buf, opts, nil
}

func (t *TextUI) ParseOptions(form url.Values) (json.RawMessage, error) {
	raw := strings.TrimSpace(form.Get("text_max_length"))
	if raw == "" {
		return ui.EncodeOptions(Options{})
	}

	maxLen, err := strconv.Atoi(raw)
	if err != nil || maxLen < 0 {
		return nil, err
	}

	return ui.EncodeOptions(Options{MaxLength: maxLen})
}

func (t *TextUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
	if ctx.Value == nil {
		return nil, nil
	}
	str, ok := ctx.Value.(string)
	if !ok {
		return ctx.Value, nil
	}
	trimmed := strings.TrimSpace(str)
	ctx.Value = trimmed
	return trimmed, nil
}

func (t *TextUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
	opts := t.decode(ctx.Field.UIMetaJSON)
	str, _ := ctx.Value.(string)
	if opts.MaxLength > 0 && len(str) > opts.MaxLength {
		return ui.ValidationResult{OK: false, Message: "Acima do limite"}
	}
	return ui.ValidationResult{OK: true}
}
