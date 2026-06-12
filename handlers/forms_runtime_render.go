package handlers

import (
	"github.com/crgimenes/devengine/auth"
	"net/http"
	"strings"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
)

// formRuntimePage is the data rendered by forms_runtime.go.tmpl.
type formRuntimePage struct {
	Authed          bool
	User            db.User
	Config          config.Config
	Form            *db.Form
	EntityType      *db.EAVEntityType
	Elements        []FormRuntimeElement
	ElementsTree    []FormRuntimeNode
	Record          *db.EAVRecord
	Values          map[string]any
	Message         string
	Error           string
	PosLoadError    string
	MenuItems       []db.MenuItemNode
	MenuMachineName string
}

// runtimeFormContext bundles what every runtime form handler loads.
type runtimeFormContext struct {
	form       *db.Form
	entityType *db.EAVEntityType // nil when the form has no EAV binding
	elements   []db.FormElement
	attributes []db.EAVAttribute
	attrMap    map[int64]*db.EAVAttribute
}

// loadRuntimeForm resolves the form named in the URL plus its entity type,
// elements and attributes. It writes the error response itself on failure.
func (h *Handlers) loadRuntimeForm(w http.ResponseWriter, r *http.Request, requireEntity bool) (runtimeFormContext, bool) {
	var ctx runtimeFormContext

	machineName := r.PathValue("machineName")
	form, err := db.Storage.GetFormByMachineName(machineName)
	if err != nil {
		h.serverError(w, r, "loadRuntimeForm", err)
		return ctx, false
	}
	if form == nil {
		h.notFound(w, r)
		return ctx, false
	}
	ctx.form = form

	if form.EAVEntityTypeID == nil {
		if requireEntity {
			h.errorPage(w, r, http.StatusBadRequest, "Form is not linked to any EAV table")
			return ctx, false
		}
	} else {
		ctx.entityType, err = db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
		if err != nil {
			h.serverError(w, r, "EAV entity type not found", err)
			return ctx, false
		}
		ctx.attributes, err = db.Storage.ListEAVAttributesByEntityTypeID(ctx.entityType.ID)
		if err != nil {
			h.serverError(w, r, "failed to list attributes", err)
			return ctx, false
		}
	}

	ctx.elements, err = db.Storage.ListFormElements(form.ID)
	if err != nil {
		h.serverError(w, r, "failed to list form elements", err)
		return ctx, false
	}

	translateFormContent(auth.RequestLocale(r), ctx.form, ctx.elements, ctx.attributes)

	ctx.attrMap = make(map[int64]*db.EAVAttribute, len(ctx.attributes))
	for i := range ctx.attributes {
		ctx.attrMap[ctx.attributes[i].ID] = &ctx.attributes[i]
	}
	return ctx, true
}

// renderRuntimeForm renders the form with the given values; per-field errors
// are attached to the element tree so the template shows each message next to
// its field.
func (h *Handlers) renderRuntimeForm(
	w http.ResponseWriter,
	r *http.Request,
	user *db.User,
	ctx runtimeFormContext,
	record *db.EAVRecord,
	values map[string]any,
	fieldErrors map[string]string,
	message, errorMsg, posLoadError string,
) {
	var runtimeElements []FormRuntimeElement
	for _, el := range ctx.elements {
		re := FormRuntimeElement{Element: el}
		if el.EAVAttributeID != nil {
			re.Attribute = ctx.attrMap[*el.EAVAttributeID]
		}
		runtimeElements = append(runtimeElements, re)
	}

	tree := BuildElementTree(ctx.elements, ctx.attrMap)
	attachFieldErrors(tree, fieldErrors)

	menuItems, menuMachineName := loadFormMenu(auth.RequestLocale(r), ctx.form)

	h.render(w, "forms_runtime.go.tmpl", formRuntimePage{
		Authed:          true,
		User:            *user,
		Config:          *h.cfg,
		Form:            ctx.form,
		EntityType:      ctx.entityType,
		Elements:        runtimeElements,
		ElementsTree:    tree,
		Record:          record,
		Values:          values,
		Message:         message,
		Error:           errorMsg,
		PosLoadError:    posLoadError,
		MenuItems:       menuItems,
		MenuMachineName: menuMachineName,
	})
}

// attachFieldErrors hangs each field's message on its tree node, recursing
// into groups, so the template needs no extra plumbing.
func attachFieldErrors(nodes []FormRuntimeNode, errs map[string]string) {
	if len(errs) == 0 {
		return
	}
	for i := range nodes {
		nodes[i].FieldError = errs[nodes[i].Element.MachineName]
		attachFieldErrors(nodes[i].Children, errs)
	}
}

// parseFormAttributesLenient parses every field, collecting per-field
// problems instead of aborting at the first one. On a bad field the raw text
// stays in values so the re-rendered form shows what the user typed.
func parseFormAttributesLenient(r *http.Request, elements []db.FormElement, attributes []db.EAVAttribute) (db.EAVRecordValues, map[string]string) {
	values := make(db.EAVRecordValues)
	fieldErrors := make(map[string]string)

	attrByID := make(map[int64]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		attrByID[attributes[i].ID] = &attributes[i]
	}

	for _, el := range elements {
		if el.EAVAttributeID == nil {
			continue
		}
		attr := attrByID[*el.EAVAttributeID]
		if attr == nil {
			continue
		}

		raw := r.FormValue(el.MachineName)
		if raw == "" {
			if attr.IsRequired {
				fieldErrors[el.MachineName] = tr(r, "required field")
			}
			values[attr.MachineName] = ""
			continue
		}

		v, err := parseElementValue(el, attr, raw)
		if err != nil {
			fieldErrors[el.MachineName] = tr(r, "invalid value")
			values[attr.MachineName] = raw
			continue
		}
		values[attr.MachineName] = v
	}
	return values, fieldErrors
}

// joinFieldErrors flattens per-field messages into the aggregated
// "Label: message; ..." form, following the element order. Used where a
// single string is needed (button JSON responses).
func joinFieldErrors(elements []db.FormElement, errs map[string]string) string {
	if len(errs) == 0 {
		return ""
	}
	var parts []string
	for _, el := range elements {
		msg, ok := errs[el.MachineName]
		if !ok || msg == "" {
			continue
		}
		label := el.Label
		if label == "" {
			label = el.MachineName
		}
		parts = append(parts, label+": "+msg)
	}
	return strings.Join(parts, "; ")
}
