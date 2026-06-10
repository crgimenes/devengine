package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/log"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/eav/ui"
	"github.com/crgimenes/devengine/filodb"
	"github.com/crgimenes/devengine/filolog"
	"github.com/crgimenes/devengine/utils"
	"github.com/crgimenes/filo"
	"github.com/crgimenes/filo/filostrings"
)

// FormRuntimeElement combines a form element with its EAV attribute info.
type FormRuntimeElement struct {
	Element   db.FormElement
	Attribute *db.EAVAttribute // nil if UI-only
}

// FormRuntimeNode represents a node in the hierarchical element tree.
type FormRuntimeNode struct {
	Element   db.FormElement
	Attribute *db.EAVAttribute
	Children  []FormRuntimeNode
}

// BuildElementTree constructs a hierarchical tree from a flat list of elements.
// Uses pointers throughout and only converts to value when building final result.
func BuildElementTree(elements []db.FormElement, attrMap map[int64]*db.EAVAttribute) []FormRuntimeNode {
	if len(elements) == 0 {
		return nil
	}

	// Build node lookup map with pointers
	nodeMap := make(map[int64]*FormRuntimeNode)
	for i := range elements {
		el := &elements[i]
		node := &FormRuntimeNode{Element: *el}
		if el.EAVAttributeID != nil {
			node.Attribute = attrMap[*el.EAVAttributeID]
		}
		nodeMap[el.ID] = node
	}

	// Build parent-child relationships using pointers
	// Children are stored as pointers for now
	childrenOf := make(map[int64][]*FormRuntimeNode)
	var rootNodes []*FormRuntimeNode

	for i := range elements {
		el := &elements[i]
		node := nodeMap[el.ID]
		if el.ParentID == nil {
			rootNodes = append(rootNodes, node)
		} else {
			childrenOf[*el.ParentID] = append(childrenOf[*el.ParentID], node)
		}
	}

	// Recursive function to build node with all descendants
	var buildNode func(n *FormRuntimeNode) FormRuntimeNode
	buildNode = func(n *FormRuntimeNode) FormRuntimeNode {
		result := FormRuntimeNode{
			Element:   n.Element,
			Attribute: n.Attribute,
		}
		// Recursively build children
		for _, childPtr := range childrenOf[n.Element.ID] {
			result.Children = append(result.Children, buildNode(childPtr))
		}
		return result
	}

	// Build roots with all their descendants
	var roots []FormRuntimeNode
	for _, rootPtr := range rootNodes {
		roots = append(roots, buildNode(rootPtr))
	}

	return roots
}

// FormsRuntimeNew shows a form for creating a new record.
func (h *Handlers) FormsRuntimeNew(w http.ResponseWriter, r *http.Request) {
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

	machineName := r.PathValue("machineName")
	form, err := db.Storage.GetFormByMachineName(machineName)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if form == nil {
		http.NotFound(w, r)
		return
	}

	// Entity type is optional - form may not be linked to EAV
	var entityType *db.EAVEntityType
	var attributes []db.EAVAttribute
	attrMap := make(map[int64]*db.EAVAttribute)

	if form.EAVEntityTypeID != nil {
		entityType, err = db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
		if err != nil {
			http.Error(w, "EAV entity type not found", http.StatusInternalServerError)
			return
		}

		// Get all attributes for this entity type
		attributes, err = db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
		if err != nil {
			http.Error(w, "failed to list attributes", http.StatusInternalServerError)
			return
		}

		// Build attribute map for lookup
		for i := range attributes {
			attrMap[attributes[i].ID] = &attributes[i]
		}
	}

	// Get form elements
	elements, err := db.Storage.ListFormElements(form.ID)
	if err != nil {
		http.Error(w, "failed to list form elements", http.StatusInternalServerError)
		return
	}

	// Build runtime elements with attribute info
	var runtimeElements []FormRuntimeElement
	for _, el := range elements {
		re := FormRuntimeElement{Element: el}
		if el.EAVAttributeID != nil {
			re.Attribute = attrMap[*el.EAVAttributeID]
		}
		runtimeElements = append(runtimeElements, re)
	}

	// Build hierarchical element tree
	elementTree := BuildElementTree(elements, attrMap)

	// Prepare initial values with defaults from attributes
	values := make(map[string]any)
	for _, attr := range attributes {
		if attr.DefaultVBool != nil {
			values[attr.MachineName] = *attr.DefaultVBool
		} else if attr.DefaultVInt != nil {
			values[attr.MachineName] = *attr.DefaultVInt
		} else if attr.DefaultVReal != nil {
			values[attr.MachineName] = *attr.DefaultVReal
		} else if attr.DefaultVText != nil {
			values[attr.MachineName] = *attr.DefaultVText
		} else if attr.DefaultVDatetime != nil {
			values[attr.MachineName] = *attr.DefaultVDatetime
		}
	}

	// Execute pos_load script (before display) - only if entity type exists
	var posLoadError string
	if entityType != nil && entityType.PosLoad != "" {
		modifiedValues, userError, execErr := db.ExecutePosLoadScript(entityType, db.EAVRecordValues(values))
		if execErr == nil {
			maps.Copy(values, modifiedValues)
			posLoadError = userError
		}
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	errorMsg := r.URL.Query().Get("error")
	if len(errorMsg) > 200 {
		errorMsg = ""
	}

	// Load menu items if form has a menu associated
	var menuItems []db.MenuItemNode
	var menuMachineName string
	if form.MenuID != nil {
		menu, err := db.Storage.GetMenuByID(*form.MenuID)
		if err == nil && menu != nil {
			menuMachineName = menu.MachineName
			items, err := db.Storage.ListMenuItems(menu.ID)
			if err == nil {
				menuItems = db.BuildMenuItemTreeWithName(items, menuMachineName)
			}
		}
	}

	data := struct {
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
	}{
		Authed:          true,
		User:            *user,
		Config:          *h.cfg,
		Form:            form,
		EntityType:      entityType,
		Elements:        runtimeElements,
		ElementsTree:    elementTree,
		Record:          nil, // New record
		Values:          values,
		Message:         message,
		Error:           errorMsg,
		PosLoadError:    posLoadError,
		MenuItems:       menuItems,
		MenuMachineName: menuMachineName,
	}

	err = h.templates(w, "forms_runtime.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// FormsRuntimeCreate handles form submission to create a new record.
func (h *Handlers) FormsRuntimeCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	machineName := r.PathValue("machineName")
	form, err := db.Storage.GetFormByMachineName(machineName)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if form == nil {
		http.NotFound(w, r)
		return
	}

	if form.EAVEntityTypeID == nil {
		http.Error(w, "Form is not linked to any EAV table", http.StatusBadRequest)
		return
	}

	entityType, err := db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
	if err != nil {
		http.Error(w, "EAV entity type not found", http.StatusInternalServerError)
		return
	}

	// Get form elements
	elements, err := db.Storage.ListFormElements(form.ID)
	if err != nil {
		http.Error(w, "failed to list form elements", http.StatusInternalServerError)
		return
	}

	// Get all attributes
	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to list attributes", http.StatusInternalServerError)
		return
	}

	// Parse form values
	parsedValues, err := parseFormAttributes(r, elements, attributes)
	if err != nil {
		http.Redirect(w, r, "/form/"+machineName+"?message="+err.Error(), http.StatusSeeOther)
		return
	}

	parsedValues, err = applyComputedExprs(r.Context(), user, attributes, parsedValues)
	if err != nil {
		log.Printf("computed_expr system error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	userErr, sysErr := evaluateValidateExprs(r.Context(), user, elements, attributes, parsedValues)
	if sysErr != nil {
		log.Printf("validate_expr system error: %v", sysErr)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if userErr != "" {
		http.Redirect(w, r, "/form/"+machineName+"?message="+userErr, http.StatusSeeOther)
		return
	}

	// Transaction
	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		http.Redirect(w, r, "/form/"+machineName+"?message=Erro ao iniciar transação", http.StatusSeeOther)
		return
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// Helper handles pre_save, record creation, vsalue saving
	_, recordRefID, err := insertRecordTx(tx, entityType, attributes, parsedValues)
	if err != nil {
		http.Redirect(w, r, "/form/"+machineName+"?message="+err.Error(), http.StatusSeeOther)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Redirect(w, r, "/form/"+machineName+"?message=Erro ao finalizar: "+err.Error(), http.StatusSeeOther)
		return
	}
	committed = true

	http.Redirect(w, r, "/form/"+machineName+"/r/"+recordRefID+"?message=Registro criado com sucesso", http.StatusSeeOther)
}

// FormsRuntimeEdit shows a form for editing an existing record.
func (h *Handlers) FormsRuntimeEdit(w http.ResponseWriter, r *http.Request) {
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

	machineName := r.PathValue("machineName")
	recordRefID := r.PathValue("recordRef")

	form, err := db.Storage.GetFormByMachineName(machineName)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if form == nil {
		http.NotFound(w, r)
		return
	}

	if form.EAVEntityTypeID == nil {
		http.Error(w, "Form is not linked to any EAV table", http.StatusBadRequest)
		return
	}

	entityType, err := db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
	if err != nil {
		http.Error(w, "EAV entity type not found", http.StatusInternalServerError)
		return
	}

	record, err := db.Storage.GetEAVRecordByRefID(recordRefID)
	if err != nil {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	// Ensure record belongs to this entity type
	if record.EntityTypeID != entityType.ID {
		http.Error(w, "Record does not belong to this form's entity type", http.StatusBadRequest)
		return
	}

	// Get form elements
	elements, err := db.Storage.ListFormElements(form.ID)
	if err != nil {
		http.Error(w, "failed to list form elements", http.StatusInternalServerError)
		return
	}

	// Get all attributes
	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to list attributes", http.StatusInternalServerError)
		return
	}

	attrMap := make(map[int64]*db.EAVAttribute)
	for i := range attributes {
		attrMap[attributes[i].ID] = &attributes[i]
	}

	// Build runtime elements
	var runtimeElements []FormRuntimeElement
	for _, el := range elements {
		re := FormRuntimeElement{Element: el}
		if el.EAVAttributeID != nil {
			re.Attribute = attrMap[*el.EAVAttributeID]
		}
		runtimeElements = append(runtimeElements, re)
	}

	// Build hierarchical element tree
	elementTree := BuildElementTree(elements, attrMap)

	// Get record values
	eavValues, err := db.Storage.GetEAVValuesByRecordID(record.ID)
	if err != nil {
		http.Error(w, "failed to list values", http.StatusInternalServerError)
		return
	}

	// Build values map
	values := make(map[string]any)
	for _, val := range eavValues {
		var attr *db.EAVAttribute
		for i := range attributes {
			if attributes[i].ID == val.AttributeID {
				attr = &attributes[i]
				break
			}
		}
		if attr == nil {
			continue
		}

		switch attr.PrimitiveKind {
		case "BOOL":
			if val.VBool != nil {
				values[attr.MachineName] = *val.VBool
			}
		case "INT":
			if val.VInt != nil {
				values[attr.MachineName] = *val.VInt
			}
		case "REAL":
			if val.VReal != nil {
				values[attr.MachineName] = *val.VReal
			}
		case "DATETIME":
			if val.VDatetime != nil {
				values[attr.MachineName] = *val.VDatetime
			}
		default:
			if val.VText != nil {
				values[attr.MachineName] = *val.VText
			}
		}
	}

	// Execute pos_load script
	var posLoadError string
	if entityType.PosLoad != "" {
		modifiedValues, userError, execErr := db.ExecutePosLoadScript(entityType, db.EAVRecordValues(values))
		if execErr == nil {
			maps.Copy(values, modifiedValues)
			posLoadError = userError
		}
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	errorMsg := r.URL.Query().Get("error")
	if len(errorMsg) > 200 {
		errorMsg = ""
	}

	// Load menu items if form has a menu associated
	var menuItems []db.MenuItemNode
	var menuMachineName string
	if form.MenuID != nil {
		menu, err := db.Storage.GetMenuByID(*form.MenuID)
		if err == nil && menu != nil {
			menuMachineName = menu.MachineName
			mItems, err := db.Storage.ListMenuItems(menu.ID)
			if err == nil {
				menuItems = db.BuildMenuItemTreeWithName(mItems, menuMachineName)
			}
		}
	}

	data := struct {
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
	}{
		Authed:          true,
		User:            *user,
		Config:          *h.cfg,
		Form:            form,
		EntityType:      entityType,
		Elements:        runtimeElements,
		ElementsTree:    elementTree,
		Record:          record,
		Values:          values,
		Message:         message,
		Error:           errorMsg,
		PosLoadError:    posLoadError,
		MenuItems:       menuItems,
		MenuMachineName: menuMachineName,
	}

	err = h.templates(w, "forms_runtime.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// FormsRuntimeUpdate handles form submission to update an existing record.
func (h *Handlers) FormsRuntimeUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	machineName := r.PathValue("machineName")
	recordRefID := r.PathValue("recordRef")

	form, err := db.Storage.GetFormByMachineName(machineName)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if form == nil {
		http.NotFound(w, r)
		return
	}

	if form.EAVEntityTypeID == nil {
		http.Error(w, "Form is not linked to any EAV table", http.StatusBadRequest)
		return
	}

	entityType, err := db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
	if err != nil {
		http.Error(w, "EAV entity type not found", http.StatusInternalServerError)
		return
	}

	record, err := db.Storage.GetEAVRecordByRefID(recordRefID)
	if err != nil {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	// Optimistic locking
	revStr := r.FormValue("rev")
	submittedRev, _ := strconv.Atoi(revStr)
	if submittedRev != record.Rev {
		http.Redirect(w, r, "/form/"+machineName+"/r/"+recordRefID+"?message=Registro foi modificado por outro usuário", http.StatusSeeOther)
		return
	}

	// Get form elements and attributes
	elements, _ := db.Storage.ListFormElements(form.ID)
	attributes, _ := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)

	// Parse form values
	parsedValues, err := parseFormAttributes(r, elements, attributes)
	if err != nil {
		http.Redirect(w, r, "/form/"+machineName+"/r/"+recordRefID+"?message="+err.Error(), http.StatusSeeOther)
		return
	}

	parsedValues, err = applyComputedExprs(r.Context(), user, attributes, parsedValues)
	if err != nil {
		log.Printf("computed_expr system error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	userErr, sysErr := evaluateValidateExprs(r.Context(), user, elements, attributes, parsedValues)
	if sysErr != nil {
		log.Printf("validate_expr system error: %v", sysErr)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if userErr != "" {
		http.Redirect(w, r, "/form/"+machineName+"/r/"+recordRefID+"?message="+userErr, http.StatusSeeOther)
		return
	}

	// Transaction
	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		http.Redirect(w, r, "/form/"+machineName+"/r/"+recordRefID+"?message=Erro ao iniciar transação", http.StatusSeeOther)
		return
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// Helper handles pre_save, revision update, value saving
	err = updateRecordTx(tx, entityType, record, attributes, parsedValues)
	if err != nil {
		http.Redirect(w, r, "/form/"+machineName+"/r/"+recordRefID+"?message="+err.Error(), http.StatusSeeOther)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Redirect(w, r, "/form/"+machineName+"/r/"+recordRefID+"?message=Erro ao finalizar: "+err.Error(), http.StatusSeeOther)
		return
	}
	committed = true

	http.Redirect(w, r, "/form/"+machineName+"/r/"+recordRefID+"?message=Registro atualizado com sucesso", http.StatusSeeOther)
}

// FormsRuntimeButtonAction handles custom button actions.
// Executes save action (if configured) and Filo code in the same transaction.
func (h *Handlers) FormsRuntimeButtonAction(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !authed {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Not authenticated"})
		return
	}

	// ... Auth check done ...

	machineName := r.PathValue("machineName")
	buttonName := r.PathValue("buttonName")

	// Get form
	form, err := db.Storage.GetFormByMachineName(machineName)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if form == nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Form not found"})
		return
	}

	// Get form elements to find the button
	elements, err := db.Storage.ListFormElements(form.ID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load elements"})
		return
	}

	// Find button element
	var button *db.FormElement
	for i := range elements {
		if elements[i].MachineName == buttonName && elements[i].ElementKind == "button" {
			button = &elements[i]
			break
		}
	}
	if button == nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Button not found"})
		return
	}

	// Delegate to core logic
	h.formsRuntimeButtonActionLogic(w, r, user, form, button, elements)
}

func (h *Handlers) formsRuntimeButtonActionLogic(w http.ResponseWriter, r *http.Request, user *db.User, form *db.Form, button *db.FormElement, elements []db.FormElement) {
	// Parse form data - handle both URL-encoded and multipart (from JavaScript FormData)
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Failed to parse multipart form: " + err.Error()})
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Failed to parse form"})
			return
		}
	}

	// Determine if this is a Create or Update context
	// record_ref_id is sent by hidden field in existing records
	recordRefID := r.FormValue("record_ref_id")
	isUpdate := recordRefID != ""
	machineName := form.MachineName

	// Ensure Entity Type is loaded if we need to access DB (RunSave or just Context)
	var entityType *db.EAVEntityType
	var err error
	if form.EAVEntityTypeID != nil {
		entityType, err = db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Entity type not found"})
			return
		}
	}

	// If RunSave is true, we need valid entity type
	if button.ButtonRunSave && entityType == nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Cannot run save action without EAV entity"})
		return
	}

	// Response variables (can be modified by Filo script)
	response := map[string]any{
		"error":       "",
		"message":     "",
		"redirect_to": "",
	}

	// =========================================================
	// 1. Prepare Data (Strict Typing)
	// =========================================================

	var parsedValues db.EAVRecordValues = make(db.EAVRecordValues)
	var attributes []db.EAVAttribute

	if form.EAVEntityTypeID != nil {
		// Get attributes
		attributes, err = db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load attributes"})
			return
		}

		// Parse form values strictly for EAV
		parsedValues, err = parseFormAttributes(r, elements, attributes)
		if err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}

	// Fetch current record if update
	var currentRecord *db.EAVRecord
	if isUpdate {
		currentRecord, err = db.Storage.GetEAVRecordByRefID(recordRefID)
		if err != nil {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Record not found"})
			return
		}
	}

	// =========================================================
	// Main Transaction
	// =========================================================
	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to begin transaction"})
		return
	}
	committed := false
	defer func() {
		if !committed {
			log.Printf("[ROLLBACK] Button action '%s' on form '%s': transaction rolled back", button.MachineName, form.MachineName)
			tx.Rollback()
		}
	}()

	// =========================================================
	// 2. Execute Filo Script (BEFORE Save)
	// =========================================================
	if button.ButtonFiloCode != "" {
		// Build Filo globals
		globals := make(map[string]filo.Value)

		// Add Form/EAV values (Strict Types)
		for k, v := range parsedValues {
			globals["field:"+k] = goToFilo(v)
		}

		// Add form metadata
		globals["form:machine_name"] = filo.VString(form.MachineName)
		globals["form:label"] = filo.VString(form.Label)
		globals["form:reference_id"] = filo.VString(form.ReferenceID)

		// Add user metadata
		if user != nil {
			globals["user:id"] = filo.VNum(float64(user.ID))
			globals["user:email"] = filo.VString(user.Email)
			globals["user:sysop"] = filo.VBool(user.Sysop)
		}

		// Add Record metadata (if available)
		globals["record:id"] = filo.VNum(0)
		globals["record:ref_id"] = filo.VString("")

		if currentRecord != nil {
			globals["record:id"] = filo.VNum(float64(currentRecord.ID))
			globals["record:ref_id"] = filo.VString(currentRecord.ReferenceID)
			globals["record:status"] = filo.VString(currentRecord.Status)
		}

		// Control variables
		globals["error"] = filo.VString("")
		globals["message"] = filo.VString("")
		globals["redirect_to"] = filo.VString("")

		// Execute Filo script
		eng := filo.NewEngine()

		// Setup DB context for Filo using the SAME TRANSACTION
		dbAdapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
		dbCtxWithTx := filodb.NewContext(dbAdapter, tx)

		// Register builtins (including DB ops attached to this TX)
		filostrings.RegisterBuiltins(eng)
		filodb.RegisterDBBuiltins(eng, dbCtxWithTx)
		filolog.RegisterLogBuiltins(eng, filolog.NewContext(globals))

		ctx := r.Context()
		cfg := filo.EvalConfig{
			StepLimit:      10000,
			RecursionLimit: 100,
			Timeout:        5 * 1e9, // 5 seconds in nanoseconds
		}

		_, newGlobals, execErr := eng.RunScript(ctx, button.ButtonFiloCode, globals, cfg)
		if execErr != nil {
			log.Printf("[ERROR] Button '%s' Filo script error: %v", button.MachineName, execErr)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Script error: " + execErr.Error()})
			return
		}

		// Read back changes to inputs from Filo
		for kRaw, val := range newGlobals {
			var goVal any
			switch val.Kind {
			case filo.KNumber:
				goVal = int64(val.Num)
				if val.Num != float64(int64(val.Num)) {
					goVal = val.Num
				}
			case filo.KString:
				goVal = val.Str
			case filo.KBool:
				goVal = val.Bool
			default:
				continue
			}

			// Handle field: prefix
			k := kRaw
			if after, ok := strings.CutPrefix(k, "field:"); ok {
				k = after
			} else {
				// Ignore non-field globals (like error, message, etc unless they match field names explicitly without prefix which is deprecated but supported for non-colliding legacy if any)
				// Actually, per strict rules, we only save "field:" variables or variables that match attribute names directly IF we supported legacy.
				// But to be safe and avoid collision with "error", "message", we ONLY map back if it matches a known attribute.
				// However, if we only inject "field:", scripts MUST write to "field:".
				// IF a script writes to "idade" (no prefix), it ends up in globals["idade"].
				// If we have a field "idade", should we accept it?
				// Risk: collision with "error".
				// Decision: Only accept "field:" prefixed variables OR variables that match attribute names BUT are not reserved words.
				// For now, let's accept both but prioritize field:?
				// To enforce the standard, let's rely on matching attribute names, but prioritize mapped Key.
			}

			if _, exists := parsedValues[k]; exists {
				parsedValues[k] = goVal
			} else {
				for _, attr := range attributes {
					if attr.MachineName == k {
						parsedValues[k] = goVal
						break
					}
				}
			}

			// Read control variables
			if kRaw == "error" && val.Kind == filo.KString && val.Str != "" {
				response["error"] = val.Str
			}
			if kRaw == "message" && val.Kind == filo.KString && val.Str != "" {
				response["message"] = val.Str
			}
			if kRaw == "redirect_to" && val.Kind == filo.KString && val.Str != "" {
				response["redirect_to"] = val.Str
			}
		}
	}

	// Check if there was an error from Filo
	if errStr, ok := response["error"].(string); ok && errStr != "" {
		jsonResponse(w, http.StatusBadRequest, response)
		return
	}

	// =========================================================
	// 3. Execute Save (AFTER Filo, using potentially modified values)
	// =========================================================
	if button.ButtonRunSave {
		if isUpdate {
			// Optimistic locking check
			revStr := r.FormValue("rev")
			submittedRev, _ := strconv.Atoi(revStr)
			if submittedRev != currentRecord.Rev {
				jsonResponse(w, http.StatusConflict, map[string]string{"error": "Record modified by another user"})
				return
			}
			if err := updateRecordTx(tx, entityType, currentRecord, attributes, parsedValues); err != nil {
				log.Printf("[ERROR] updateRecordTx failed: %v", err)
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
		} else {
			var newRefID string
			_, newRefID, err = insertRecordTx(tx, entityType, attributes, parsedValues)
			if err != nil {
				log.Printf("[ERROR] insertRecordTx failed: %v", err)
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			recordRefID = newRefID
			response["redirect_to"] = "/form/" + machineName + "/r/" + newRefID
		}
	}

	// 4. Commit Transaction
	if err := tx.Commit(); err != nil {
		log.Printf("[ERROR] Commit failed: %v", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Commit failed: " + err.Error()})
		return
	}
	committed = true

	jsonResponse(w, http.StatusOK, response)
}

// jsonResponse writes a JSON response with the given status code.
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.Encode(data)
}

// FormsRuntimeActionsJS serves the dynamically generated JavaScript for button actions.
// This allows the JS to be loaded as an external file, complying with CSP.
func (h *Handlers) FormsRuntimeActionsJS(w http.ResponseWriter, r *http.Request) {
	machineName := r.PathValue("machineName")

	// Get form
	form, err := db.Storage.GetFormByMachineName(machineName)
	if err != nil {
		http.Error(w, "// internal server error", http.StatusInternalServerError)
		return
	}
	if form == nil {
		http.Error(w, "// Form not found", http.StatusNotFound)
		return
	}

	// Get form elements to find buttons
	elements, err := db.Storage.ListFormElements(form.ID)
	if err != nil {
		http.Error(w, "// Failed to load elements", http.StatusInternalServerError)
		return
	}

	// Set content type as JavaScript
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Generate JavaScript
	var js strings.Builder
	js.WriteString("// Auto-generated button actions for form: ")
	js.WriteString(form.MachineName)
	js.WriteString("\nwindow.FormButtonActions = {\n")

	first := true
	for _, el := range elements {
		if el.ElementKind == "button" && el.ButtonJSCode != "" {
			if !first {
				js.WriteString(",\n")
			}
			first = false
			js.WriteString("    '")
			js.WriteString(el.MachineName)
			js.WriteString("': function(formData, button) {\n        ")
			js.WriteString(el.ButtonJSCode)
			js.WriteString("\n    }")
		}
	}

	js.WriteString("\n};\n")

	w.Write([]byte(js.String()))
}

// Unused import placeholder
var _ = sql.ErrNoRows

// =========================================================
// Helper Functions for Atomic Transactions
// =========================================================

// parseFormAttributes extracts and strictly parses EAV values from the request.
func parseFormAttributes(r *http.Request, elements []db.FormElement, attributes []db.EAVAttribute) (db.EAVRecordValues, error) {
	attrMap := make(map[int64]*db.EAVAttribute)
	for i := range attributes {
		attrMap[attributes[i].ID] = &attributes[i]
	}

	parsedValues := make(db.EAVRecordValues)
	for _, el := range elements {
		if el.EAVAttributeID == nil {
			continue // UI-only element
		}
		attr := attrMap[*el.EAVAttributeID]
		if attr == nil {
			continue
		}

		fieldName := el.MachineName
		rawValue := r.FormValue(fieldName)

		if rawValue == "" {
			if attr.IsRequired {
				return nil, fmt.Errorf("campo obrigatório: %s", attr.Label)
			}
			parsedValues[attr.MachineName] = ""
			continue
		}

		v, err := parseElementValue(el, attr, rawValue)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", attr.Label, err)
		}
		parsedValues[attr.MachineName] = v
	}
	return parsedValues, nil
}

// parseElementValue prefers a registered ui.FieldUI for the element's UIKind
// and falls back to the primitive-kind switch when no plugin is registered.
// This is the only interpretation point of raw form values during create/update.
func parseElementValue(el db.FormElement, attr *db.EAVAttribute, raw string) (any, error) {
	plugin, ok := ui.Get(el.UIKind)
	if ok {
		if !plugin.HasPersistence() {
			return raw, nil
		}
		opts := plugin.ParseOptions(el.UIMetaJSON)
		v, err := plugin.Parse(raw, opts)
		if err != nil {
			return nil, err
		}
		err = plugin.Validate(v, opts)
		if err != nil {
			return nil, err
		}
		return v, nil
	}

	switch attr.PrimitiveKind {
	case "BOOL":
		return raw == "true" || raw == "1", nil
	case "INT":
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return raw, nil
		}
		return v, nil
	case "REAL":
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return raw, nil
		}
		return v, nil
	}
	return raw, nil
}

// insertRecordTx handles the creation of a new record within a transaction.
// Executes pre_save script, inserts record, and inserts values.
func insertRecordTx(tx *db.Transaction, entityType *db.EAVEntityType, attributes []db.EAVAttribute, values db.EAVRecordValues) (int64, string, error) {
	// Execute pre_save script with transaction context
	if entityType.PreSave != "" {
		dbAdapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
		dbCtxWithTx := filodb.NewContext(dbAdapter, tx)

		scriptSetup := func(eng *filo.Engine) {
			filostrings.RegisterBuiltins(eng)
			filodb.RegisterDBBuiltins(eng, dbCtxWithTx)
		}
		modifiedValues, userError, execErr := db.ExecutePreSaveScriptWithSetup(entityType, values, scriptSetup)
		if execErr != nil {
			return 0, "", fmt.Errorf("erro no script pre_save: %w", execErr)
		}
		if userError != "" {
			return 0, "", fmt.Errorf("%s", userError)
		}
		values = modifiedValues
	}

	// Create record
	refID := utils.NewOpaqueID()
	recordID, recordRefID, err := tx.CreateEAVRecordInTx(refID, entityType.ID, "active")
	if err != nil {
		return 0, "", fmt.Errorf("erro ao criar registro: %w", err)
	}

	// Save values
	if err := tx.SaveEAVValuesTx(recordID, attributes, values); err != nil {
		return 0, "", err
	}

	return recordID, recordRefID, nil
}

// updateRecordTx handles the update of an existing record within a transaction.
// Executes pre_save script, updates record revision, and inserts/replaces values.
func updateRecordTx(tx *db.Transaction, entityType *db.EAVEntityType, record *db.EAVRecord, attributes []db.EAVAttribute, values db.EAVRecordValues) error {
	// Execute pre_save script
	if entityType.PreSave != "" {
		dbAdapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
		dbCtxWithTx := filodb.NewContext(dbAdapter, tx)

		scriptSetup := func(eng *filo.Engine) {
			filostrings.RegisterBuiltins(eng)
			filodb.RegisterDBBuiltins(eng, dbCtxWithTx)
		}
		modifiedValues, userError, execErr := db.ExecutePreSaveScriptWithSetup(entityType, values, scriptSetup)
		if execErr != nil {
			return fmt.Errorf("erro no script pre_save: %w", execErr)
		}
		if userError != "" {
			return fmt.Errorf("%s", userError)
		}
		values = modifiedValues
	}

	// Update record rev
	if err := tx.UpdateEAVRecordRevInTx(record.ID); err != nil {
		return fmt.Errorf("erro ao atualizar registro: %w", err)
	}

	// Save values
	return tx.SaveEAVValuesTx(record.ID, attributes, values)
}
