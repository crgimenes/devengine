package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/log"
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return nil, nil, nil, false
	}
	if form == nil {
		http.NotFound(w, r)
		return nil, nil, nil, false
	}
	if form.EAVEntityTypeID == nil {
		http.Error(w, "form has no entity type", http.StatusBadRequest)
		return nil, nil, nil, false
	}
	et, err := db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
	if err != nil {
		http.Error(w, "entity type not found", http.StatusInternalServerError)
		return nil, nil, nil, false
	}
	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil {
		log.Printf("ListEAVAttributesByEntityTypeID: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return nil, nil, nil, false
	}
	return form, et, attributes, true
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
		log.Printf("fetchRecordRows: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

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

	err = h.templates(w, "forms_runtime_list.go.tmpl", data)
	if err != nil {
		log.Printf("template error in forms_runtime_list.go.tmpl: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
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
	err = h.templates(w, "forms_runtime_list_rows", data)
	if err != nil {
		log.Printf("template error in forms_runtime_list_rows: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
