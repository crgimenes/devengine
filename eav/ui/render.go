package ui

import (
	"bytes"
	"fmt"

	"github.com/crgimenes/devengine/templates"
)

func RenderTemplateBuffer(name string, data any) (string, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func RenderEditTemplate(ui FieldUI, ctx TemplateData) (string, error) {
	name := fmt.Sprintf("eav_ui_%s_edit", ui.ID())
	return RenderTemplateBuffer(name, ctx)
}

func RenderViewTemplate(ui FieldUI, ctx TemplateData) (string, error) {
	name := fmt.Sprintf("eav_ui_%s_view", ui.ID())
	return RenderTemplateBuffer(name, ctx)
}
