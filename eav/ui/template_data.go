package ui

import "html/template"

type ChildItem struct {
	ColumnWidth int
	Content     template.HTML
}

type ChildRow struct {
	Children []ChildItem
}

type TemplateData struct {
	Context          FieldRuntimeContext
	Options          any
	InputName        string
	Error            string
	ReadOnly         bool
	Display          string
	Value            any
	ChildrenRowsEdit []ChildRow
	ChildrenRowsView []ChildRow
	ParentAccordion  string
}
