package handlers

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/eav/ui"
)

// FormWithEntityType combines a form with its linked entity type info.
type FormWithEntityType struct {
	Form           db.Form
	EntityTypeName string // Name of linked EAV entity type, empty if none
}

// ToolsForms shows the forms list page.
func (h *Handlers) ToolsForms(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, // check auth
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "ToolsForms", err)
		return
	}

	if !authed {
		return
	}

	// Sysop-only check
	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		h.errorPage(w, r, http.StatusBadRequest, "message too long")
		return
	}

	// Get all forms
	forms, err := db.Storage.ListForms()
	if err != nil {
		h.serverError(w, r, "list forms", err)
		return
	}

	// Build forms with entity type names
	var formsWithTypes []FormWithEntityType
	for _, f := range forms {
		fwt := FormWithEntityType{Form: f}
		if f.EAVEntityTypeID != nil {
			et, err := db.Storage.GetEAVEntityTypeByID(*f.EAVEntityTypeID)
			if err == nil {
				fwt.EntityTypeName = et.Name
			}
		}
		formsWithTypes = append(formsWithTypes, fwt)
	}

	data := struct {
		Authed      bool
		User        db.User
		Locale      string
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Forms       []FormWithEntityType
	}{
		Authed:      true,
		Locale:      auth.RequestLocale(r),
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "forms",
		Forms:       formsWithTypes,
	}

	h.render(w, "tools_forms.go.tmpl", data)
}

// ToolsFormsNew shows the new form creation page.
func (h *Handlers) ToolsFormsNew(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	// Get entity types for dropdown
	entityTypes, err := db.Storage.ListEAVEntityTypes()
	if err != nil {
		h.serverError(w, r, "failed to list entity types", err)
		return
	}

	data := struct {
		Authed      bool
		User        db.User
		Locale      string
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		EntityTypes []db.EAVEntityType
	}{
		Authed:      true,
		Locale:      auth.RequestLocale(r),
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "forms",
		EntityTypes: entityTypes,
	}

	h.render(w, "tools_forms_new.go.tmpl", data)
}

// ToolsFormsCreate handles form creation.
func (h *Handlers) ToolsFormsCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	description := r.FormValue("description")
	entityTypeRefID := r.FormValue("entity_type_id")

	// Validate required fields
	if machineName == "" || label == "" {
		http.Redirect(w, r, "/tools/forms/new?message="+tr(r, "Name and label are required"), http.StatusSeeOther)
		return
	}

	// Get entity type ID if selected
	var eavEntityTypeID *int64
	if entityTypeRefID != "" {
		et, err := db.Storage.GetEAVEntityTypeByRefID(entityTypeRefID)
		if err != nil {
			http.Redirect(w, r, "/tools/forms/new?message="+tr(r, "EAV table not found"), http.StatusSeeOther)
			return
		}
		eavEntityTypeID = &et.ID
	}

	// Create form
	form, err := db.Storage.CreateForm(machineName, label, description, eavEntityTypeID)
	if err != nil {
		ref := logRef("ToolsFormsCreate", err)
		http.Redirect(w, r, "/tools/forms/new?message="+tr(r, "Could not create the form (ref %s)", ref), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+form.ReferenceID+"/edit?message="+tr(r, "Form created successfully"), http.StatusSeeOther)
}

// ToolsFormsEdit shows the form edit page.
func (h *Handlers) ToolsFormsEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	// Get linked entity type info
	var entityType *db.EAVEntityType
	if form.EAVEntityTypeID != nil {
		entityType, _ = db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
	}

	// Get entity types for dropdown
	entityTypes, _ := db.Storage.ListEAVEntityTypes()

	// Get menus for dropdown
	menus, _ := db.Storage.ListMenus()

	// Get form elements and sort hierarchically
	elements, _ := db.Storage.ListFormElements(form.ID)
	elements = db.SortElementsHierarchically(elements)

	// Get EAV attributes if linked to entity type
	var eavAttributes []db.EAVAttribute
	attrMap := make(map[int64]string) // ID -> Label
	if form.EAVEntityTypeID != nil {
		eavAttributes, _ = db.Storage.ListEAVAttributesByEntityTypeID(*form.EAVEntityTypeID)
		for _, attr := range eavAttributes {
			attrMap[attr.ID] = attr.Label
		}
	}

	// Build element ID -> Label map for parent labels
	elementMap := make(map[int64]string)  // ID -> Label or MachineName
	parentIDMap := make(map[int64]*int64) // ID -> ParentID
	for _, el := range elements {
		label := el.Label
		if label == "" {
			label = el.MachineName
		}
		elementMap[el.ID] = label
		parentIDMap[el.ID] = el.ParentID
	}

	// Calculate depth for each element
	var getDepth func(id int64) int
	getDepth = func(id int64) int {
		parentID := parentIDMap[id]
		if parentID == nil {
			return 0
		}
		return 1 + getDepth(*parentID)
	}

	// Build enriched elements with attribute labels, parent info, and depth
	type ElementWithAttr struct {
		db.FormElement
		EAVAttributeLabel string
		ParentLabel       string
		Depth             int
	}
	var enrichedElements []ElementWithAttr
	for _, el := range elements {
		e := ElementWithAttr{FormElement: el, Depth: getDepth(el.ID)}
		if el.EAVAttributeID != nil {
			e.EAVAttributeLabel = attrMap[*el.EAVAttributeID]
		}
		if el.ParentID != nil {
			e.ParentLabel = elementMap[*el.ParentID]
		}
		enrichedElements = append(enrichedElements, e)
	}

	// Dereference MenuID for template comparison
	var selectedMenuID int64
	if form.MenuID != nil {
		selectedMenuID = *form.MenuID
	}

	data := struct {
		Authed         bool
		User           db.User
		Locale         string
		Error          string
		Message        string
		Config         config.Config
		CurrentPage    string
		Form           *db.Form
		EntityType     *db.EAVEntityType
		EntityTypes    []db.EAVEntityType
		Elements       []ElementWithAttr
		EAVAttributes  []db.EAVAttribute
		Menus          []db.Menu
		SelectedMenuID int64
	}{
		Authed:         true,
		Locale:         auth.RequestLocale(r),
		User:           *user,
		Message:        message,
		Config:         *h.cfg,
		CurrentPage:    "forms",
		Form:           form,
		EntityType:     entityType,
		EntityTypes:    entityTypes,
		Elements:       enrichedElements,
		EAVAttributes:  eavAttributes,
		Menus:          menus,
		SelectedMenuID: selectedMenuID,
	}

	h.render(w, "tools_forms_edit.go.tmpl", data)
}

// ToolsFormsUpdate handles form update.
func (h *Handlers) ToolsFormsUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	description := r.FormValue("description")
	entityTypeRefID := r.FormValue("entity_type_id")
	menuRefID := r.FormValue("menu_id")
	hideSubmitButton := r.FormValue("hide_submit_button") == "on"
	hideCancelButton := r.FormValue("hide_cancel_button") == "on"
	hideTitle := r.FormValue("hide_title") == "on"
	showSystemInfo := r.FormValue("show_system_info") == "on"
	isSearch := r.FormValue("is_search") == "on"
	exposeAPI := r.FormValue("expose_api") == "on"

	// Validate required fields
	if machineName == "" || label == "" {
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message="+tr(r, "Name and label are required"), http.StatusSeeOther)
		return
	}

	// Get entity type ID if selected
	var eavEntityTypeID *int64
	if entityTypeRefID != "" {
		et, err := db.Storage.GetEAVEntityTypeByRefID(entityTypeRefID)
		if err != nil {
			http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message="+tr(r, "EAV table not found"), http.StatusSeeOther)
			return
		}
		eavEntityTypeID = &et.ID
	}

	// Get menu ID if selected
	var menuID *int64
	if menuRefID != "" {
		menu, err := db.Storage.GetMenuByRefID(menuRefID)
		if err != nil {
			http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message="+tr(r, "Menu not found"), http.StatusSeeOther)
			return
		}
		menuID = &menu.ID
	}

	// Update form
	err = db.Storage.UpdateForm(form.ID, machineName, label, description, eavEntityTypeID, hideSubmitButton, hideCancelButton, hideTitle, showSystemInfo, menuID, isSearch, exposeAPI)
	if err != nil {
		ref := logRef("ToolsFormsUpdate", err)
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message="+tr(r, "Could not update (ref %s)", ref), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message="+tr(r, "Form updated successfully"), http.StatusSeeOther)
}

// ToolsFormsDelete handles form deletion.
func (h *Handlers) ToolsFormsDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	err = db.Storage.SoftDeleteForm(form.ID)
	if err != nil {
		ref := logRef("ToolsFormsDelete", err)
		http.Redirect(w, r, "/tools/forms?message="+tr(r, "Could not delete (ref %s)", ref), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms?message="+tr(r, "Form deleted successfully"), http.StatusSeeOther)
}

// ToolsFormsElementCreate handles creating a new form element.
func (h *Handlers) ToolsFormsElementCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	machineName := r.FormValue("machine_name")
	elementKind := r.FormValue("element_kind")
	label := r.FormValue("label")
	helpText := r.FormValue("help_text")
	uiKind := r.FormValue("ui_kind")
	eavAttrRefID := r.FormValue("eav_attribute_id")
	isUIOnlyStr := r.FormValue("is_ui_only")

	// Validate
	if machineName == "" {
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message="+tr(r, "Element name is required"), http.StatusSeeOther)
		return
	}

	// Get EAV attribute ID if selected
	var eavAttrID *int64
	if eavAttrRefID != "" {
		attr, err := db.Storage.GetEAVAttributeByRefID(eavAttrRefID)
		if err != nil || form.EAVEntityTypeID == nil || attr.EntityTypeID != *form.EAVEntityTypeID {
			h.errorPage(w, r, http.StatusBadRequest, "Attribute does not belong to this form's entity type")
			return
		}
		eavAttrID = &attr.ID
	}

	isUIOnly := isUIOnlyStr == "1" || isUIOnlyStr == "true"

	// Get next z_order
	elements, _ := db.Storage.ListFormElements(form.ID)
	zOrder := len(elements)

	_, err = db.Storage.CreateFormElement(
		form.ID, nil, machineName, elementKind, label, helpText,
		zOrder, 12, uiKind, "", eavAttrID, isUIOnly, false,
	)
	if err != nil {
		ref := logRef("ToolsFormsElementCreate", err)
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message="+tr(r, "Could not create the element (ref %s)", ref), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message="+tr(r, "Element added"), http.StatusSeeOther)
}

// ToolsFormsElementDelete handles deleting a form element.
func (h *Handlers) ToolsFormsElementDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	if element.FormID != form.ID {
		h.errorPage(w, r, http.StatusBadRequest, "Element does not belong to this form")
		return
	}

	err = db.Storage.DeleteFormElement(element.ID)
	if err != nil {
		h.serverError(w, r, "delete element", err)
		return
	}

	// Check if HTMX request
	if r.Header.Get("HX-Request") == "true" {
		h.renderElementsTableRows(w, r, formRefID)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message="+tr(r, "Element removed"), http.StatusSeeOther)
}

// ToolsFormsElementMoveUp moves an element up in the z_order within its parent.
func (h *Handlers) ToolsFormsElementMoveUp(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	if element.FormID != form.ID {
		h.errorPage(w, r, http.StatusBadRequest, "Element does not belong to this form")
		return
	}

	err = db.Storage.MoveElementUp(element.ID)
	if err != nil {
		h.serverError(w, r, "move element", err)
		return
	}

	// Check if HTMX request
	if r.Header.Get("HX-Request") == "true" {
		h.renderElementsTableRows(w, r, formRefID)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit", http.StatusSeeOther)
}

// ToolsFormsElementMoveDown moves an element down in the z_order within its parent.
func (h *Handlers) ToolsFormsElementMoveDown(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	if element.FormID != form.ID {
		h.errorPage(w, r, http.StatusBadRequest, "Element does not belong to this form")
		return
	}

	err = db.Storage.MoveElementDown(element.ID)
	if err != nil {
		h.serverError(w, r, "move element", err)
		return
	}

	// Check if HTMX request
	if r.Header.Get("HX-Request") == "true" {
		h.renderElementsTableRows(w, r, formRefID)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit", http.StatusSeeOther)
}

// renderElementsTableRows renders just the table rows for HTMX partial updates.
func (h *Handlers) renderElementsTableRows(w http.ResponseWriter, r *http.Request, formRefID string) {
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
		return
	}

	elements, _ := db.Storage.ListFormElements(form.ID)
	elements = db.SortElementsHierarchically(elements)

	// Get parent labels and parent ID map for depth calculation
	parentMap := make(map[int64]string)
	parentIDMap := make(map[int64]*int64)
	for _, el := range elements {
		label := el.Label
		if label == "" {
			label = el.MachineName
		}
		parentMap[el.ID] = label
		parentIDMap[el.ID] = el.ParentID
	}

	// Calculate depth for each element
	var getDepth func(id int64) int
	getDepth = func(id int64) int {
		parentID := parentIDMap[id]
		if parentID == nil {
			return 0
		}
		return 1 + getDepth(*parentID)
	}

	var eavAttributes []db.EAVAttribute
	attrMap := make(map[int64]string)
	if form.EAVEntityTypeID != nil {
		eavAttributes, _ = db.Storage.ListEAVAttributesByEntityTypeID(*form.EAVEntityTypeID)
		for _, attr := range eavAttributes {
			attrMap[attr.ID] = attr.Label
		}
	}

	type ElementView struct {
		db.FormElement
		ParentLabel       string
		EAVAttributeLabel string
		Depth             int
	}

	var elementViews []ElementView
	for _, el := range elements {
		ev := ElementView{FormElement: el, Depth: getDepth(el.ID)}
		if el.ParentID != nil {
			ev.ParentLabel = parentMap[*el.ParentID]
		}
		if el.EAVAttributeID != nil {
			ev.EAVAttributeLabel = attrMap[*el.EAVAttributeID]
		}
		elementViews = append(elementViews, ev)
	}

	data := struct {
		Locale   string
		Form     *db.Form
		Elements []ElementView
	}{
		Locale:   auth.RequestLocale(r),
		Form:     form,
		Elements: elementViews,
	}

	h.render(w, "elements_table_rows", data)
}

// ToolsFormsElementEdit shows the form element edit page.
func (h *Handlers) ToolsFormsElementEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	if element.FormID != form.ID {
		h.errorPage(w, r, http.StatusBadRequest, "Element does not belong to this form")
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	// Get entity type info
	var entityType *db.EAVEntityType
	var eavAttributes []db.EAVAttribute
	if form.EAVEntityTypeID != nil {
		entityType, _ = db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
		eavAttributes, _ = db.Storage.ListEAVAttributesByEntityTypeID(*form.EAVEntityTypeID)
	}

	// Get group elements for parent dropdown (exclude self)
	groupElements, _ := db.Storage.ListGroupElements(form.ID)
	var filteredGroups []db.FormElement
	for _, g := range groupElements {
		if g.ID != element.ID {
			filteredGroups = append(filteredGroups, g)
		}
	}

	// Get all elements for counting (for z_order suggestion)
	allElements, _ := db.Storage.ListFormElements(form.ID)

	// Find parent element label
	var parentLabel string
	if element.ParentID != nil {
		for _, el := range allElements {
			if el.ID == *element.ParentID {
				parentLabel = el.Label
				if parentLabel == "" {
					parentLabel = el.MachineName
				}
				break
			}
		}
	}

	// Find EAV attribute label
	var eavAttributeLabel string
	if element.EAVAttributeID != nil {
		for _, attr := range eavAttributes {
			if attr.ID == *element.EAVAttributeID {
				eavAttributeLabel = attr.Label
				break
			}
		}
	}

	// Datalists for the reference/subform options panel: every entity type
	// plus the union of attribute machine names.
	entityTypes, err := db.Storage.ListEAVEntityTypes()
	if err != nil {
		h.serverError(w, r, "ListEAVEntityTypes", err)
		return
	}
	nameSet := map[string]bool{}
	for _, et := range entityTypes {
		attrs, aerr := db.Storage.ListEAVAttributesByEntityTypeID(et.ID)
		if aerr != nil {
			continue
		}
		for _, a := range attrs {
			nameSet[a.MachineName] = true
		}
	}
	attributeNames := make([]string, 0, len(nameSet))
	for n := range nameSet {
		attributeNames = append(attributeNames, n)
	}
	sort.Strings(attributeNames)

	data := struct {
		Authed            bool
		User              db.User
		Locale            string
		Error             string
		Message           string
		Config            config.Config
		CurrentPage       string
		Form              *db.Form
		Element           *db.FormElement
		EntityType        *db.EAVEntityType
		EAVAttributes     []db.EAVAttribute
		EntityTypes       []db.EAVEntityType
		AttributeNames    []string
		GroupElements     []db.FormElement
		AllElements       []db.FormElement
		ParentLabel       string
		EAVAttributeLabel string
		UIKinds           []string
	}{
		Authed:            true,
		Locale:            auth.RequestLocale(r),
		User:              *user,
		Message:           message,
		Config:            *h.cfg,
		CurrentPage:       "forms",
		Form:              form,
		Element:           element,
		EntityType:        entityType,
		EAVAttributes:     eavAttributes,
		EntityTypes:       entityTypes,
		AttributeNames:    attributeNames,
		GroupElements:     filteredGroups,
		AllElements:       allElements,
		ParentLabel:       parentLabel,
		EAVAttributeLabel: eavAttributeLabel,
		UIKinds:           ui.IDs(),
	}

	h.render(w, "tools_forms_element_edit.go.tmpl", data)
}

// ToolsFormsElementUpdate handles form element update.
func (h *Handlers) ToolsFormsElementUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	if element.FormID != form.ID {
		h.errorPage(w, r, http.StatusBadRequest, "Element does not belong to this form")
		return
	}

	// Parse form values
	machineName := r.FormValue("machine_name")
	elementKind := r.FormValue("element_kind")
	label := r.FormValue("label")
	helpText := r.FormValue("help_text")
	parentRefID := r.FormValue("parent_id")
	zOrderStr := r.FormValue("z_order")
	colSpanStr := r.FormValue("col_span")
	uiKind := r.FormValue("ui_kind")
	uiMetaJSON := r.FormValue("ui_meta_json")
	eavAttrRefID := r.FormValue("eav_attribute_id")
	isUIOnlyStr := r.FormValue("is_ui_only")
	isReadonlyStr := r.FormValue("is_readonly")
	hideLabelStr := r.FormValue("hide_label")
	hideHelpTextStr := r.FormValue("hide_help_text")

	// Button-specific fields
	validateExpr := r.FormValue("validate_expr")
	buttonFiloCode := r.FormValue("button_filo_code")
	buttonRunSaveStr := r.FormValue("button_run_save")
	buttonJSCode := r.FormValue("button_js_code")
	buttonStyle := r.FormValue("button_style")
	buttonConfirmMsg := r.FormValue("button_confirm_msg")

	// Validate required fields
	if machineName == "" {
		message := url.QueryEscape(tr(r, "Machine name is required"))
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/elements/"+elementRefID+"/edit?message="+message, http.StatusSeeOther)
		return
	}

	// Parse z_order
	zOrder, _ := strconv.Atoi(zOrderStr)
	if zOrder < 0 {
		zOrder = 0
	}

	// Parse col_span
	colSpan, _ := strconv.Atoi(colSpanStr)
	if colSpan < 1 || colSpan > 12 {
		colSpan = 12
	}

	// Parse alignment
	alignment := r.FormValue("alignment")
	if alignment != "left" && alignment != "center" && alignment != "right" {
		alignment = "left"
	}

	// Get parent element ID
	var parentID *int64
	if parentRefID != "" {
		parentEl, err := db.Storage.GetFormElementByRefID(parentRefID)
		if err != nil || parentEl.FormID != form.ID || parentEl.ID == element.ID {
			h.errorPage(w, r, http.StatusBadRequest, "Invalid parent element")
			return
		}
		parentID = &parentEl.ID
	}

	// Get EAV attribute ID
	var eavAttrID *int64
	if eavAttrRefID != "" {
		attr, err := db.Storage.GetEAVAttributeByRefID(eavAttrRefID)
		if err != nil || form.EAVEntityTypeID == nil || attr.EntityTypeID != *form.EAVEntityTypeID {
			h.errorPage(w, r, http.StatusBadRequest, "Attribute does not belong to this form's entity type")
			return
		}
		eavAttrID = &attr.ID
	}

	isUIOnly := isUIOnlyStr == "1" || isUIOnlyStr == "on"
	isReadonly := isReadonlyStr == "1" || isReadonlyStr == "on"
	hideLabel := hideLabelStr == "1" || hideLabelStr == "on"
	hideHelpText := hideHelpTextStr == "1" || hideHelpTextStr == "on"
	buttonRunSave := buttonRunSaveStr == "1" || buttonRunSaveStr == "on"

	err = db.Storage.UpdateFormElement(
		element.ID,
		parentID,
		machineName, elementKind, label, helpText,
		zOrder, colSpan,
		alignment,
		uiKind, uiMetaJSON,
		eavAttrID,
		isUIOnly, isReadonly, hideLabel, hideHelpText,
		validateExpr,
		buttonFiloCode, buttonRunSave, buttonJSCode, buttonStyle, buttonConfirmMsg,
	)
	if err != nil {
		ref := logRef("ToolsFormsElementUpdate", err)
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/elements/"+elementRefID+"/edit?message="+tr(r, "Could not update (ref %s)", ref), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message="+tr(r, "Element updated"), http.StatusSeeOther)
}

// ToolsFormsRecords shows the records of the form's linked entity_type. The
// initial batch is rendered inline; subsequent batches arrive via HTMX
// against ToolsFormsRecordsRows.
func (h *Handlers) ToolsFormsRecords(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	form, entityType, err := loadFormAndEntity(r, w)
	if err != nil || form == nil {
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		h.serverError(w, r, "Failed to list attributes", err)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	rows, nextCursor, err := h.fetchRecordRows(entityType.ID, 0, q, attributes)
	if err != nil {
		h.serverError(w, r, "Failed to list records", err)
		return
	}
	resolveReferenceDisplays(form, attributes, rows)

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed          bool
		User            db.User
		Locale          string
		Config          config.Config
		CurrentPage     string
		Form            *db.Form
		EntityType      *db.EAVEntityType
		Attributes      []db.EAVAttribute
		Rows            []RecordRow
		NextCursor      int64
		Filter          string
		ColCount        int
		Message         string
		FormRefID       string
		EntityTypeRefID string
	}{
		Authed:          true,
		Locale:          auth.RequestLocale(r),
		User:            *user,
		Config:          *h.cfg,
		CurrentPage:     "forms",
		Form:            form,
		EntityType:      entityType,
		Attributes:      attributes,
		Rows:            rows,
		NextCursor:      nextCursor,
		Filter:          q,
		ColCount:        len(attributes) + 2,
		Message:         message,
		FormRefID:       form.ReferenceID,
		EntityTypeRefID: entityType.ReferenceID,
	}

	h.render(w, "tools_forms_records.go.tmpl", data)
}
