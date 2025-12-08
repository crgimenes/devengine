package list

import (
	"embed"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

type ListUI struct{}

type Options struct {
	Values []string `json:"values,omitempty"`
}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("list", func() ui.FieldUI {
		return &ListUI{}
	}, templatesFS)
}

func (l *ListUI) ID() string {
	return "list"
}

func (l *ListUI) Label() string {
	return "Lista"
}

func (l *ListUI) HasPersistence() bool {
	return true
}

func (l *ListUI) SupportsReadOnly() bool {
	return true
}

func (l *ListUI) IsGroupingField() bool {
	return false
}

func (l *ListUI) RecommendedPrimitiveKind() string {
	return "TEXT"
}

func (l *ListUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (l *ListUI) decode(meta string) Options {
	opts := Options{}
	err := ui.DecodeOptions(meta, &opts)
	if err != nil {
		return Options{}
	}
	return opts
}

func (l *ListUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	opts := l.decode(ctx.Field.UIMetaJSON)
	data := ui.TemplateData{Context: ctx, Options: opts}
	html, err := ui.RenderTemplateBuffer("eav_ui_list_options", data)
	if err != nil {
		return "", nil, err
	}
	return html, opts, nil
}

func (l *ListUI) ParseOptions(form url.Values) (json.RawMessage, error) {
	raw := strings.TrimSpace(form.Get("list_values"))
	values := []string{}
	if raw != "" {
		parts := strings.Split(raw, ",")
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed == "" {
				continue
			}
			values = append(values, trimmed)
		}
	}
	return ui.EncodeOptions(Options{Values: values})
}

func (l *ListUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
	return ctx.Value, nil
}

func (l *ListUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
	opts := l.decode(ctx.Field.UIMetaJSON)
	if len(opts.Values) == 0 {
		return ui.ValidationResult{OK: true}
	}
	str, _ := ctx.Value.(string)
	if str == "" {
		return ui.ValidationResult{OK: true}
	}
	for _, opt := range opts.Values {
		if strings.EqualFold(opt, str) {
			return ui.ValidationResult{OK: true}
		}
	}
	return ui.ValidationResult{OK: false, Message: "Valor não permitido"}
}
