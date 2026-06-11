package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
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
		true,  // check auth
		false, // check ratelimit
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !authed {
		return
	}

	// Sysop-only check
	if !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		http.Error(w, "message too long", http.StatusBadRequest)
		return
	}

	// Get all forms
	forms, err := db.Storage.ListForms()
	if err != nil {
		http.Error(w, "failed to list forms: "+err.Error(), http.StatusInternalServerError)
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
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Forms       []FormWithEntityType
	}{
		Authed:      true,
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
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	// Get entity types for dropdown
	entityTypes, err := db.Storage.ListEAVEntityTypes()
	if err != nil {
		http.Error(w, "failed to list entity types", http.StatusInternalServerError)
		return
	}

	data := struct {
		Authed      bool
		User        db.User
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		EntityTypes []db.EAVEntityType
	}{
		Authed:      true,
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
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	description := r.FormValue("description")
	entityTypeRefID := r.FormValue("entity_type_id")

	// Validate required fields
	if machineName == "" || label == "" {
		http.Redirect(w, r, "/tools/forms/new?message=Nome e Label são obrigatórios", http.StatusSeeOther)
		return
	}

	// Get entity type ID if selected
	var eavEntityTypeID *int64
	if entityTypeRefID != "" {
		et, err := db.Storage.GetEAVEntityTypeByRefID(entityTypeRefID)
		if err != nil {
			http.Redirect(w, r, "/tools/forms/new?message=Tabela EAV não encontrada", http.StatusSeeOther)
			return
		}
		eavEntityTypeID = &et.ID
	}

	// Create form
	form, err := db.Storage.CreateForm(machineName, label, description, eavEntityTypeID)
	if err != nil {
		http.Redirect(w, r, "/tools/forms/new?message=Erro ao criar formulário: "+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+form.ReferenceID+"/edit?message=Formulário criado com sucesso", http.StatusSeeOther)
}

// ToolsFormsEdit shows the form edit page.
func (h *Handlers) ToolsFormsEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
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
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
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

	// Validate required fields
	if machineName == "" || label == "" {
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Nome e Label são obrigatórios", http.StatusSeeOther)
		return
	}

	// Get entity type ID if selected
	var eavEntityTypeID *int64
	if entityTypeRefID != "" {
		et, err := db.Storage.GetEAVEntityTypeByRefID(entityTypeRefID)
		if err != nil {
			http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Tabela EAV não encontrada", http.StatusSeeOther)
			return
		}
		eavEntityTypeID = &et.ID
	}

	// Get menu ID if selected
	var menuID *int64
	if menuRefID != "" {
		menu, err := db.Storage.GetMenuByRefID(menuRefID)
		if err != nil {
			http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Menu não encontrado", http.StatusSeeOther)
			return
		}
		menuID = &menu.ID
	}

	// Update form
	err = db.Storage.UpdateForm(form.ID, machineName, label, description, eavEntityTypeID, hideSubmitButton, hideCancelButton, hideTitle, showSystemInfo, menuID)
	if err != nil {
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Erro ao atualizar: "+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Formulário atualizado com sucesso", http.StatusSeeOther)
}

// ToolsFormsDelete handles form deletion.
func (h *Handlers) ToolsFormsDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
		return
	}

	err = db.Storage.SoftDeleteForm(form.ID)
	if err != nil {
		http.Redirect(w, r, "/tools/forms?message=Erro ao excluir: "+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms?message=Formulário excluído com sucesso", http.StatusSeeOther)
}

// ToolsFormsElementCreate handles creating a new form element.
func (h *Handlers) ToolsFormsElementCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
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
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Nome do elemento é obrigatório", http.StatusSeeOther)
		return
	}

	// Get EAV attribute ID if selected
	var eavAttrID *int64
	if eavAttrRefID != "" {
		attr, err := db.Storage.GetEAVAttributeByRefID(eavAttrRefID)
		if err == nil {
			eavAttrID = &attr.ID
		}
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
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Erro ao criar elemento: "+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Elemento adicionado", http.StatusSeeOther)
}

// ToolsFormsElementDelete handles deleting a form element.
func (h *Handlers) ToolsFormsElementDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		http.Error(w, "Elemento não encontrado", http.StatusNotFound)
		return
	}

	err = db.Storage.DeleteFormElement(element.ID)
	if err != nil {
		http.Error(w, "Erro ao excluir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if HTMX request
	if r.Header.Get("HX-Request") == "true" {
		h.renderElementsTableRows(w, formRefID)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Elemento removido", http.StatusSeeOther)
}

// ToolsFormsElementMoveUp moves an element up in the z_order within its parent.
func (h *Handlers) ToolsFormsElementMoveUp(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		http.Error(w, "Elemento não encontrado", http.StatusNotFound)
		return
	}

	if err := db.Storage.MoveElementUp(element.ID); err != nil {
		http.Error(w, "Erro ao mover: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if HTMX request
	if r.Header.Get("HX-Request") == "true" {
		h.renderElementsTableRows(w, formRefID)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit", http.StatusSeeOther)
}

// ToolsFormsElementMoveDown moves an element down in the z_order within its parent.
func (h *Handlers) ToolsFormsElementMoveDown(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		http.Error(w, "Elemento não encontrado", http.StatusNotFound)
		return
	}

	if err := db.Storage.MoveElementDown(element.ID); err != nil {
		http.Error(w, "Erro ao mover: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if HTMX request
	if r.Header.Get("HX-Request") == "true" {
		h.renderElementsTableRows(w, formRefID)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit", http.StatusSeeOther)
}

// renderElementsTableRows renders just the table rows for HTMX partial updates.
func (h *Handlers) renderElementsTableRows(w http.ResponseWriter, formRefID string) {
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
		Form     *db.Form
		Elements []ElementView
	}{
		Form:     form,
		Elements: elementViews,
	}

	h.render(w, "elements_table_rows", data)
}

// ToolsFormsElementEdit shows the form element edit page.
func (h *Handlers) ToolsFormsElementEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
		return
	}

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		http.Error(w, "Element not found", http.StatusNotFound)
		return
	}

	// Verify element belongs to this form
	if element.FormID != form.ID {
		http.Error(w, "Element does not belong to this form", http.StatusBadRequest)
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

	data := struct {
		Authed            bool
		User              db.User
		Error             string
		Message           string
		Config            config.Config
		CurrentPage       string
		Form              *db.Form
		Element           *db.FormElement
		EntityType        *db.EAVEntityType
		EAVAttributes     []db.EAVAttribute
		GroupElements     []db.FormElement
		AllElements       []db.FormElement
		ParentLabel       string
		EAVAttributeLabel string
	}{
		Authed:            true,
		User:              *user,
		Message:           message,
		Config:            *h.cfg,
		CurrentPage:       "forms",
		Form:              form,
		Element:           element,
		EntityType:        entityType,
		EAVAttributes:     eavAttributes,
		GroupElements:     filteredGroups,
		AllElements:       allElements,
		ParentLabel:       parentLabel,
		EAVAttributeLabel: eavAttributeLabel,
	}

	h.render(w, "tools_forms_element_edit.go.tmpl", data)
}

// ToolsFormsElementUpdate handles form element update.
func (h *Handlers) ToolsFormsElementUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
		return
	}

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		http.Error(w, "Element not found", http.StatusNotFound)
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
	buttonFiloCode := r.FormValue("button_filo_code")
	buttonRunSaveStr := r.FormValue("button_run_save")
	buttonJSCode := r.FormValue("button_js_code")
	buttonStyle := r.FormValue("button_style")
	buttonConfirmMsg := r.FormValue("button_confirm_msg")

	// Validate required fields
	if machineName == "" {
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/elements/"+elementRefID+"/edit?message=Nome é obrigatório", http.StatusSeeOther)
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
		if err == nil && parentEl.FormID == form.ID {
			parentID = &parentEl.ID
		}
	}

	// Get EAV attribute ID
	var eavAttrID *int64
	if eavAttrRefID != "" {
		attr, err := db.Storage.GetEAVAttributeByRefID(eavAttrRefID)
		if err == nil {
			eavAttrID = &attr.ID
		}
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
		buttonFiloCode, buttonRunSave, buttonJSCode, buttonStyle, buttonConfirmMsg,
	)
	if err != nil {
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/elements/"+elementRefID+"/edit?message=Erro ao atualizar: "+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Elemento atualizado", http.StatusSeeOther)
}

// ToolsFormsRecords shows the records of the form's linked entity_type. The
// initial batch is rendered inline; subsequent batches arrive via HTMX
// against ToolsFormsRecordsRows.
func (h *Handlers) ToolsFormsRecords(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	form, entityType, err := loadFormAndEntity(r, w)
	if err != nil || form == nil {
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "Failed to list attributes", http.StatusInternalServerError)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	rows, nextCursor, err := h.fetchRecordRows(entityType.ID, 0, q, attributes)
	if err != nil {
		http.Error(w, "Failed to list records", http.StatusInternalServerError)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed          bool
		User            db.User
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
