package group

import (
	"embed"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/crgimenes/devengine/eav/ui"
)

type GroupUI struct{}

type Options struct {
	Subtitle string `json:"subtitle,omitempty"`
	Style    string `json:"style,omitempty"`
}

//go:embed templates/*.go.tmpl
var templatesFS embed.FS

func init() {
	ui.RegisterFieldUI("group", func() ui.FieldUI {
		return &GroupUI{}
	}, templatesFS)
}

func (g *GroupUI) ID() string {
	return "group"
}

func (g *GroupUI) Label() string {
	return "Grupo"
}

func (g *GroupUI) HasPersistence() bool {
	return false
}

func (g *GroupUI) SupportsReadOnly() bool {
	return false
}

func (g *GroupUI) IsGroupingField() bool {
	return true
}

func (g *GroupUI) RecommendedPrimitiveKind() string {
	return "-"
}

func (g *GroupUI) TemplatesFS() embed.FS {
	return templatesFS
}

func (g *GroupUI) decode(meta string) Options {
	opts := Options{}
	err := ui.DecodeOptions(meta, &opts)
	if err != nil {
		return Options{}
	}
	return opts
}

func (g *GroupUI) RenderOptions(ctx ui.FieldRuntimeContext) (string, any, error) {
	opts := g.decode(ctx.Field.UIMetaJSON)
	data := ui.TemplateData{Context: ctx, Options: opts}
	buf, err := ui.RenderTemplateBuffer("eav_ui_group_options", data)
	if err != nil {
		return "", nil, err
	}
	return buf, opts, nil
}

func (g *GroupUI) ParseOptions(form url.Values) (json.RawMessage, error) {
	subtitle := strings.TrimSpace(form.Get("group_subtitle"))
	style := strings.TrimSpace(form.Get("group_style"))

	opts := Options{}
	if subtitle != "" {
		opts.Subtitle = subtitle
	}
	if style != "" {
		opts.Style = style
	}

	return ui.EncodeOptions(opts)
}

func (g *GroupUI) BeforeSave(ctx *ui.FieldRuntimeContext) (any, error) {
	return nil, nil
}

func (g *GroupUI) Validate(ctx ui.FieldRuntimeContext) ui.ValidationResult {
	return ui.ValidationResult{OK: true}
}
