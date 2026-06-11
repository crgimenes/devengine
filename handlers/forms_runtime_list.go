package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/log"
	"github.com/crgimenes/devengine/templates"
)

// loadFormMenu resolves the navbar menu bound to a form, if any. Returns the
// item tree and the menu machine_name (empty when the form has no menu).
func loadFormMenu(form *db.Form) ([]db.MenuItemNode, string) {
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
	return db.BuildMenuItemTreeWithName(items, menu.MachineName), menu.MachineName
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
	return form, et, attributes, true
}

// resolveReferenceDisplays swaps raw reference ids for their display labels
// in listing rows, using the same choices source as the form select. Failures
// leave the raw ids in place — the listing still works.
func resolveReferenceDisplays(form *db.Form, attributes []db.EAVAttribute, rows []RecordRow) {
	if len(rows) == 0 {
		return
	}
	elements, err := db.Storage.ListFormElements(form.ID)
	if err != nil {
		return
	}
	attrByID := make(map[int64]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		attrByID[attributes[i].ID] = &attributes[i]
	}
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
		for i := range rows {
			v, ok := rows[i].Values[attr.MachineName].(string)
			if !ok {
				continue
			}
			label, ok := byValue[v]
			if ok {
				rows[i].Values[attr.MachineName] = label
			}
		}
	}
}

// FormsRuntimeList renders the end-user listing of a form's records with the
// ordered, cursor-based infinite scroll. Each row links to the runtime editor
// and a "Novo" button points at the create view.
func (h *Handlers) FormsRuntimeList(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
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

	menuItems, menuMachineName := loadFormMenu(form)

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed          bool
		User            db.User
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
		true, false, true,
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
		Form       *db.Form
		Attributes []db.EAVAttribute
		Rows       []RecordRow
		NextCursor int64
		Filter     string
		ColCount   int
	}{
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
