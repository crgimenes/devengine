package handlers

import (
	"net/http"
	"strconv"

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
		http.Error(w, "Forbidden", http.StatusForbidden)
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

	err = h.templates(w, "tools_forms.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// ToolsFormsNew shows the new form creation page.
func (h *Handlers) ToolsFormsNew(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
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

	err = h.templates(w, "tools_forms_new.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// ToolsFormsCreate handles form creation.
func (h *Handlers) ToolsFormsCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
		http.Error(w, "Forbidden", http.StatusForbidden)
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

	// Get form elements
	elements, _ := db.Storage.ListFormElements(form.ID)

	// Get EAV attributes if linked to entity type
	var eavAttributes []db.EAVAttribute
	attrMap := make(map[int64]string) // ID -> Label
	if form.EAVEntityTypeID != nil {
		eavAttributes, _ = db.Storage.ListEAVAttributesByEntityTypeID(*form.EAVEntityTypeID)
		for _, attr := range eavAttributes {
			attrMap[attr.ID] = attr.Label
		}
	}

	// Build enriched elements with attribute labels
	type ElementWithAttr struct {
		db.FormElement
		EAVAttributeLabel string
	}
	var enrichedElements []ElementWithAttr
	for _, el := range elements {
		e := ElementWithAttr{FormElement: el}
		if el.EAVAttributeID != nil {
			e.EAVAttributeLabel = attrMap[*el.EAVAttributeID]
		}
		enrichedElements = append(enrichedElements, e)
	}

	data := struct {
		Authed        bool
		User          db.User
		Error         string
		Message       string
		Config        config.Config
		CurrentPage   string
		Form          *db.Form
		EntityType    *db.EAVEntityType
		EntityTypes   []db.EAVEntityType
		Elements      []ElementWithAttr
		EAVAttributes []db.EAVAttribute
	}{
		Authed:        true,
		User:          *user,
		Message:       message,
		Config:        *h.cfg,
		CurrentPage:   "forms",
		Form:          form,
		EntityType:    entityType,
		EntityTypes:   entityTypes,
		Elements:      enrichedElements,
		EAVAttributes: eavAttributes,
	}

	err = h.templates(w, "tools_forms_edit.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// ToolsFormsUpdate handles form update.
func (h *Handlers) ToolsFormsUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
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

	// Update form
	err = db.Storage.UpdateForm(form.ID, machineName, label, description, eavEntityTypeID)
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
		http.Error(w, "Forbidden", http.StatusForbidden)
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

// ToolsFormsTest opens the form in test mode (preview).
func (h *Handlers) ToolsFormsTest(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
		return
	}

	// Get form elements
	elements, _ := db.Storage.ListFormElements(form.ID)

	// Get linked entity type and attributes
	var entityType *db.EAVEntityType
	var eavAttributes []db.EAVAttribute
	if form.EAVEntityTypeID != nil {
		entityType, _ = db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
		eavAttributes, _ = db.Storage.ListEAVAttributesByEntityTypeID(*form.EAVEntityTypeID)
	}

	data := struct {
		Authed        bool
		User          db.User
		Config        config.Config
		CurrentPage   string
		Form          *db.Form
		EntityType    *db.EAVEntityType
		Elements      []db.FormElement
		EAVAttributes []db.EAVAttribute
		IsTestMode    bool
	}{
		Authed:        true,
		User:          *user,
		Config:        *h.cfg,
		CurrentPage:   "forms",
		Form:          form,
		EntityType:    entityType,
		Elements:      elements,
		EAVAttributes: eavAttributes,
		IsTestMode:    true,
	}

	err = h.templates(w, "tools_forms_test.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// ToolsFormsElementCreate handles creating a new form element.
func (h *Handlers) ToolsFormsElementCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
		zOrder, uiKind, "", eavAttrID, isUIOnly, false,
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
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	elementRefID := r.PathValue("element_id")

	element, err := db.Storage.GetFormElementByRefID(elementRefID)
	if err != nil {
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Elemento não encontrado", http.StatusSeeOther)
		return
	}

	err = db.Storage.DeleteFormElement(element.ID)
	if err != nil {
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Erro ao excluir: "+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Elemento removido", http.StatusSeeOther)
}

// ToolsFormsRecords shows the list of EAV records for the form's linked entity type.
func (h *Handlers) ToolsFormsRecords(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
		return
	}

	if form.EAVEntityTypeID == nil {
		http.Redirect(w, r, "/tools/forms/"+formRefID+"/edit?message=Formulário não possui tabela EAV vinculada", http.StatusSeeOther)
		return
	}

	entityType, err := db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
	if err != nil {
		http.Error(w, "Entity type not found", http.StatusInternalServerError)
		return
	}

	// Get records for this entity type
	records, _, err := db.Storage.ListEAVRecordsByEntityTypeID(entityType.ID, 50, 0)
	if err != nil {
		http.Error(w, "Failed to list records", http.StatusInternalServerError)
		return
	}

	// Get attributes for column display
	attributes, _ := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)

	// Get values for each record (first few attrs as preview)
	type RecordPreview struct {
		Record db.EAVRecord
		Values map[string]interface{}
	}
	var recordPreviews []RecordPreview
	for _, rec := range records {
		vals, _ := db.Storage.GetEAVValuesByRecordID(rec.ID)
		valMap := make(map[string]interface{})
		for _, v := range vals {
			for _, attr := range attributes {
				if attr.ID == v.AttributeID {
					switch attr.PrimitiveKind {
					case "BOOL":
						if v.VBool != nil {
							valMap[attr.MachineName] = *v.VBool
						}
					case "INT":
						if v.VInt != nil {
							valMap[attr.MachineName] = *v.VInt
						}
					case "REAL":
						if v.VReal != nil {
							valMap[attr.MachineName] = *v.VReal
						}
					case "TEXT":
						if v.VText != nil {
							valMap[attr.MachineName] = *v.VText
						}
					default:
						if v.VText != nil {
							valMap[attr.MachineName] = *v.VText
						}
					}
					break
				}
			}
		}
		recordPreviews = append(recordPreviews, RecordPreview{Record: rec, Values: valMap})
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed      bool
		User        db.User
		Config      config.Config
		CurrentPage string
		Form        *db.Form
		EntityType  *db.EAVEntityType
		Records     []RecordPreview
		Attributes  []db.EAVAttribute
		Message     string
	}{
		Authed:      true,
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "forms",
		Form:        form,
		EntityType:  entityType,
		Records:     recordPreviews,
		Attributes:  attributes,
		Message:     message,
	}

	err = h.templates(w, "tools_forms_records.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// Removed unused import strconv by adding _ placeholder
var _ = strconv.Atoi
