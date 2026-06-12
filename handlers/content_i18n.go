package handlers

import (
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
)

// translateFormContent swaps user-content texts (form title, element labels
// and help, attribute labels) for their translations in the given locale.
// Runtime screens only: authoring screens keep the original text.
func translateFormContent(loc string, form *db.Form, elements []db.FormElement, attributes []db.EAVAttribute) {
	if form != nil {
		form.Label = i18n.ContentOr(loc, form.ReferenceID, "label", form.Label)
	}
	for i := range elements {
		elements[i].Label = i18n.ContentOr(loc, elements[i].ReferenceID, "label", elements[i].Label)
		elements[i].HelpText = i18n.ContentOr(loc, elements[i].ReferenceID, "help_text", elements[i].HelpText)
	}
	for i := range attributes {
		attributes[i].Label = i18n.ContentOr(loc, attributes[i].ReferenceID, "label", attributes[i].Label)
	}
}

// translateMenuNodes swaps menu item labels for their translations.
func translateMenuNodes(loc string, nodes []db.MenuItemNode) {
	for i := range nodes {
		nodes[i].Label = i18n.ContentOr(loc, nodes[i].ReferenceID, "label", nodes[i].Label)
		translateMenuNodes(loc, nodes[i].Children)
	}
}
