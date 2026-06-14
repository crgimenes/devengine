package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/log"
	"github.com/crgimenes/devengine/templates"
)

// loadFormMenu resolves the navbar menu bound to a form, if any. Returns the
// item tree and the menu machine_name (empty when the form has no menu).
func loadFormMenu(loc string, form *db.Form) ([]db.MenuItemNode, string) {
	if form.MenuID == nil {
		return nil, ""
	}
	menu, err := db.Storage.GetMenuByID(*form.MenuID)
	if err != nil || menu == nil {
		return nil, ""
	}
	items, err := db.Storage.ListMenuItems(menu.ID)
	if err != nil {
		return nil, menu.MachineName
	}
	nodes := db.BuildMenuItemTreeWithName(items, menu.MachineName)
	translateMenuNodes(loc, nodes)
	return nodes, menu.MachineName
}

// formListContext loads the form (by machine_name), its bound entity type and
// attributes. Any authenticated user may reach it; binding to an entity type is
// required to list records.
func (h *Handlers) formListContext(w http.ResponseWriter, r *http.Request) (*db.Form, *db.EAVEntityType, []db.EAVAttribute, bool) {
	machineName := r.PathValue("machineName")
	form, err := db.Storage.GetFormByMachineName(machineName)
	if err != nil {
		h.serverError(w, r, "formListContext", err)
		return nil, nil, nil, false
	}
	if form == nil {
		h.notFound(w, r)
		return nil, nil, nil, false
	}
	if form.EAVEntityTypeID == nil {
		h.errorPage(w, r, http.StatusBadRequest, "form has no entity type")
		return nil, nil, nil, false
	}
	et, err := db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
	if err != nil {
		h.serverError(w, r, "entity type not found", err)
		return nil, nil, nil, false
	}
	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil {
		h.serverError(w, r, "formListContext", err)
		return nil, nil, nil, false
	}
	loc := auth.RequestLocale(r)
	attributes = listColumns(loc, form, attributes)
	translateFormContent(loc, form, nil, attributes)
	return form, et, attributes, true
}

// referenceLabelMaps returns, per reference attribute machine name, the map
// from stored record refID to its display label, using the same choices
// source as the form select. Lookup failures simply yield smaller maps — the
// caller keeps the raw ids.
func referenceLabelMaps(form *db.Form, attributes []db.EAVAttribute) map[string]map[string]string {
	elements, err := db.Storage.ListFormElements(form.ID)
	if err != nil {
		return nil
	}
	attrByID := make(map[int64]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		attrByID[attributes[i].ID] = &attributes[i]
	}
	out := make(map[string]map[string]string)
	for _, el := range elements {
		if el.UIKind != "reference" || el.EAVAttributeID == nil {
			continue
		}
		attr := attrByID[*el.EAVAttributeID]
		if attr == nil {
			continue
		}
		var meta struct {
			Entity  string `json:"entity"`
			Display string `json:"display"`
		}
		err = json.Unmarshal([]byte(el.UIMetaJSON), &meta)
		if err != nil || meta.Entity == "" {
			continue
		}
		byValue := make(map[string]string)
		for _, c := range templates.ReferenceChoices(meta.Entity, meta.Display) {
			byValue[c.Value] = c.Label
		}
		out[attr.MachineName] = byValue
	}
	return out
}

// listColumns restricts the listing to the attributes bound to the form's
// elements, in element order, so each form over an entity is its own view.
// Column headers take the ELEMENT label (translated): the form names its own
// columns. A form without bound fields keeps every attribute.
func listColumns(loc string, form *db.Form, attributes []db.EAVAttribute) []db.EAVAttribute {
	elements, err := db.Storage.ListFormElements(form.ID)
	if err != nil {
		return attributes
	}
	attrByID := make(map[int64]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		attrByID[attributes[i].ID] = &attributes[i]
	}
	var out []db.EAVAttribute
	seen := make(map[int64]bool)
	for _, el := range elements {
		if el.EAVAttributeID == nil || seen[*el.EAVAttributeID] {
			continue
		}
		attr := attrByID[*el.EAVAttributeID]
		if attr == nil {
			continue
		}
		seen[*el.EAVAttributeID] = true
		col := *attr
		if el.Label != "" {
			col.Label = i18n.ContentOr(loc, el.ReferenceID, "label", el.Label)
		}
		out = append(out, col)
	}
	if len(out) == 0 {
		return attributes
	}
	return out
}

// resolveReferenceDisplays swaps raw reference ids for their display labels
// in listing rows. Failures leave the raw ids in place — the listing still
// works.
func resolveReferenceDisplays(form *db.Form, attributes []db.EAVAttribute, rows []RecordRow) {
	if len(rows) == 0 {
		return
	}
	for machineName, byValue := range referenceLabelMaps(form, attributes) {
		for i := range rows {
			v, ok := rows[i].Values[machineName].(string)
			if !ok {
				continue
			}
			label, ok := byValue[v]
			if ok {
				rows[i].Values[machineName] = label
			}
		}
	}
}

// FormsRuntimeList renders the end-user listing of a form's records with the
// ordered, cursor-based infinite scroll. Each row links to the runtime editor
// and a "New" button points at the create view.
func (h *Handlers) FormsRuntimeList(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "FormsRuntimeList", err)
		return
	}
	if !authed {
		return
	}

	form, et, attributes, ok := h.formListContext(w, r)
	if !ok {
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	rows, nextCursor, err := h.fetchRecordRows(et.ID, 0, q, attributes)
	if err != nil {
		h.serverError(w, r, "FormsRuntimeList", err)
		return
	}

	resolveReferenceDisplays(form, attributes, rows)

	menuItems, menuMachineName := loadFormMenu(auth.RequestLocale(r), form)

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed          bool
		User            db.User
		Locale          string
		Config          config.Config
		Form            *db.Form
		Attributes      []db.EAVAttribute
		Rows            []RecordRow
		NextCursor      int64
		Filter          string
		ColCount        int
		Message         string
		MenuItems       []db.MenuItemNode
		MenuMachineName string
	}{
		Authed:          true,
		Locale:          auth.RequestLocale(r),
		User:            *user,
		Config:          *h.cfg,
		Form:            form,
		Attributes:      attributes,
		Rows:            rows,
		NextCursor:      nextCursor,
		Filter:          q,
		ColCount:        len(attributes) + 1,
		Message:         message,
		MenuItems:       menuItems,
		MenuMachineName: menuMachineName,
	}

	h.render(w, "forms_runtime_list.go.tmpl", data)
}

// FormsRuntimeListRows renders only the rows + next sentinel for the HTMX
// infinite scroll on the end-user listing.
func (h *Handlers) FormsRuntimeListRows(w http.ResponseWriter, r *http.Request) {
	_, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed {
		return
	}

	form, et, attributes, ok := h.formListContext(w, r)
	if !ok {
		return
	}

	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	rows, nextCursor, err := h.fetchRecordRows(et.ID, cursor, q, attributes)
	if err != nil {
		log.Printf("fetchRecordRows: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resolveReferenceDisplays(form, attributes, rows)

	data := struct {
		Locale     string
		Form       *db.Form
		Attributes []db.EAVAttribute
		Rows       []RecordRow
		NextCursor int64
		Filter     string
		ColCount   int
	}{
		Locale:     auth.RequestLocale(r),
		Form:       form,
		Attributes: attributes,
		Rows:       rows,
		NextCursor: nextCursor,
		Filter:     q,
		ColCount:   len(attributes) + 1,
	}

	// The file is define-only; executing it by filename renders nothing, so
	// address the defined template directly.
	h.render(w, "forms_runtime_list_rows", data)
}
