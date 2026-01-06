package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/filodb"
	"github.com/crgimenes/devengine/utils"
	"github.com/crgimenes/filo"
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
func BuildElementTree(elements []db.FormElement, attrMap map[int64]*db.EAVAttribute) []FormRuntimeNode {
	// Build node lookup map (using pointers to allow modification)
	nodeMap := make(map[int64]*FormRuntimeNode)
	for i := range elements {
		el := &elements[i]
		node := &FormRuntimeNode{Element: *el}
		if el.EAVAttributeID != nil {
			node.Attribute = attrMap[*el.EAVAttributeID]
		}
		nodeMap[el.ID] = node
	}

	// First pass: build parent-child relationships
	for i := range elements {
		el := &elements[i]
		if el.ParentID != nil {
			parentNode, ok := nodeMap[*el.ParentID]
			if ok {
				childNode := nodeMap[el.ID]
				parentNode.Children = append(parentNode.Children, *childNode)
			}
		}
	}

	// Second pass: collect root elements (those without parent)
	var roots []FormRuntimeNode
	for i := range elements {
		el := &elements[i]
		if el.ParentID == nil {
			node := nodeMap[el.ID]
			roots = append(roots, *node)
		}
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

	formRefID := r.PathValue("formRef")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
		return
	}

	// Form must be linked to an EAV entity type
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

	// Get all attributes for this entity type
	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to list attributes", http.StatusInternalServerError)
		return
	}

	// Build attribute map for lookup
	attrMap := make(map[int64]*db.EAVAttribute)
	for i := range attributes {
		attrMap[attributes[i].ID] = &attributes[i]
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
	values := make(map[string]interface{})
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

	// Execute pos_load script (before display)
	var posLoadError string
	if entityType.PosLoad != "" {
		modifiedValues, userError, execErr := db.ExecutePosLoadScript(entityType, db.EAVRecordValues(values))
		if execErr == nil {
			for k, v := range modifiedValues {
				values[k] = v
			}
			posLoadError = userError
		}
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed       bool
		User         db.User
		Config       config.Config
		Form         *db.Form
		EntityType   *db.EAVEntityType
		Elements     []FormRuntimeElement
		ElementsTree []FormRuntimeNode
		Record       *db.EAVRecord
		Values       map[string]interface{}
		Message      string
		PosLoadError string
	}{
		Authed:       true,
		User:         *user,
		Config:       *h.cfg,
		Form:         form,
		EntityType:   entityType,
		Elements:     runtimeElements,
		ElementsTree: elementTree,
		Record:       nil, // New record
		Values:       values,
		Message:      message,
		PosLoadError: posLoadError,
	}

	err = h.templates(w, "forms_runtime.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// FormsRuntimeCreate handles form submission to create a new record.
func (h *Handlers) FormsRuntimeCreate(w http.ResponseWriter, r *http.Request) {
	_, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("formRef")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
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

	attrMap := make(map[int64]*db.EAVAttribute)
	for i := range attributes {
		attrMap[attributes[i].ID] = &attributes[i]
	}

	// Parse form values from elements
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

		// Handle empty values for non-required fields
		if rawValue == "" {
			if attr.IsRequired {
				http.Redirect(w, r, "/forms/"+formRefID+"/new?message=Campo obrigatório: "+attr.Label, http.StatusSeeOther)
				return
			}
			parsedValues[attr.MachineName] = ""
			continue
		}

		// Parse based on primitive kind
		switch attr.PrimitiveKind {
		case "BOOL":
			parsedValues[attr.MachineName] = rawValue == "true" || rawValue == "1"
		case "INT":
			if v, err := strconv.ParseInt(rawValue, 10, 64); err == nil {
				parsedValues[attr.MachineName] = v
			} else {
				parsedValues[attr.MachineName] = rawValue
			}
		case "REAL":
			if v, err := strconv.ParseFloat(rawValue, 64); err == nil {
				parsedValues[attr.MachineName] = v
			} else {
				parsedValues[attr.MachineName] = rawValue
			}
		default:
			parsedValues[attr.MachineName] = rawValue
		}
	}

	// =========================================================
	// Transaction-wrapped pre_save script + record creation
	// =========================================================
	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		http.Redirect(w, r, "/forms/"+formRefID+"/new?message=Erro ao iniciar transação", http.StatusSeeOther)
		return
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// Execute pre_save script with transaction context
	if entityType.PreSave != "" {
		dbAdapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
		dbCtxWithTx := filodb.NewContext(dbAdapter, &txAdapter{tx: tx})

		scriptSetup := func(eng *filo.Engine) {
			filo.RegisterStringBuiltins(eng)
			filodb.RegisterDBBuiltins(eng, dbCtxWithTx)
		}
		modifiedValues, userError, execErr := db.ExecutePreSaveScriptWithSetup(entityType, parsedValues, scriptSetup)
		if execErr != nil {
			http.Redirect(w, r, "/forms/"+formRefID+"/new?message=Erro no script pre_save: "+execErr.Error(), http.StatusSeeOther)
			return
		}
		if userError != "" {
			http.Redirect(w, r, "/forms/"+formRefID+"/new?message="+userError, http.StatusSeeOther)
			return
		}
		parsedValues = modifiedValues
	}

	// Create record using transaction
	refID := utils.NewOpaqueID()
	var recordID int64
	var recordRefID string
	err = tx.QueryRow(`INSERT INTO eav_records (reference_id, entity_type_id, status, rev) VALUES (?, ?, 'active', 1) RETURNING id, reference_id`,
		refID, entityType.ID).Scan(&recordID, &recordRefID)
	if err != nil {
		http.Redirect(w, r, "/forms/"+formRefID+"/new?message=Erro ao criar registro: "+err.Error(), http.StatusSeeOther)
		return
	}

	// Save each value using transaction
	for machineName, rawValue := range parsedValues {
		var attr *db.EAVAttribute
		for i := range attributes {
			if attributes[i].MachineName == machineName {
				attr = &attributes[i]
				break
			}
		}
		if attr == nil || attr.IsComputed {
			continue
		}

		var vBool, vInt, vReal, vText, vDatetime interface{}
		switch attr.PrimitiveKind {
		case "BOOL":
			if b, ok := rawValue.(bool); ok {
				vBool = b
			}
		case "INT":
			if i, ok := rawValue.(int64); ok {
				vInt = i
			}
		case "REAL":
			if f, ok := rawValue.(float64); ok {
				vReal = f
			}
		case "DATETIME":
			if s, ok := rawValue.(string); ok && s != "" {
				vDatetime = s
			}
		default: // TEXT
			if s, ok := rawValue.(string); ok {
				vText = s
			}
		}

		err = tx.Exec(`INSERT OR REPLACE INTO eav_values (record_id, attribute_id, v_bool, v_int, v_real, v_text, v_datetime)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			recordID, attr.ID, vBool, vInt, vReal, vText, vDatetime)
		if err != nil {
			http.Redirect(w, r, "/forms/"+formRefID+"/new?message=Erro ao salvar valor: "+err.Error(), http.StatusSeeOther)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Redirect(w, r, "/forms/"+formRefID+"/new?message=Erro ao finalizar: "+err.Error(), http.StatusSeeOther)
		return
	}
	committed = true

	http.Redirect(w, r, "/forms/"+formRefID+"/r/"+recordRefID+"?message=Registro criado com sucesso", http.StatusSeeOther)
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

	formRefID := r.PathValue("formRef")
	recordRefID := r.PathValue("recordRef")

	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
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
	values := make(map[string]interface{})
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
			for k, v := range modifiedValues {
				values[k] = v
			}
			posLoadError = userError
		}
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed       bool
		User         db.User
		Config       config.Config
		Form         *db.Form
		EntityType   *db.EAVEntityType
		Elements     []FormRuntimeElement
		ElementsTree []FormRuntimeNode
		Record       *db.EAVRecord
		Values       map[string]interface{}
		Message      string
		PosLoadError string
	}{
		Authed:       true,
		User:         *user,
		Config:       *h.cfg,
		Form:         form,
		EntityType:   entityType,
		Elements:     runtimeElements,
		ElementsTree: elementTree,
		Record:       record,
		Values:       values,
		Message:      message,
		PosLoadError: posLoadError,
	}

	err = h.templates(w, "forms_runtime.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// FormsRuntimeUpdate handles form submission to update an existing record.
func (h *Handlers) FormsRuntimeUpdate(w http.ResponseWriter, r *http.Request) {
	_, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	formRefID := r.PathValue("formRef")
	recordRefID := r.PathValue("recordRef")

	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
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
		http.Redirect(w, r, "/forms/"+formRefID+"/r/"+recordRefID+"?message=Registro foi modificado por outro usuário", http.StatusSeeOther)
		return
	}

	// Get form elements and attributes
	elements, _ := db.Storage.ListFormElements(form.ID)
	attributes, _ := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)

	attrMap := make(map[int64]*db.EAVAttribute)
	for i := range attributes {
		attrMap[attributes[i].ID] = &attributes[i]
	}

	// Parse form values
	parsedValues := make(db.EAVRecordValues)
	for _, el := range elements {
		if el.EAVAttributeID == nil {
			continue
		}
		attr := attrMap[*el.EAVAttributeID]
		if attr == nil {
			continue
		}

		fieldName := el.MachineName
		rawValue := r.FormValue(fieldName)

		if rawValue == "" {
			if attr.IsRequired {
				http.Redirect(w, r, "/forms/"+formRefID+"/r/"+recordRefID+"?message=Campo obrigatório: "+attr.Label, http.StatusSeeOther)
				return
			}
			parsedValues[attr.MachineName] = ""
			continue
		}

		switch attr.PrimitiveKind {
		case "BOOL":
			parsedValues[attr.MachineName] = rawValue == "true" || rawValue == "1"
		case "INT":
			if v, err := strconv.ParseInt(rawValue, 10, 64); err == nil {
				parsedValues[attr.MachineName] = v
			} else {
				parsedValues[attr.MachineName] = rawValue
			}
		case "REAL":
			if v, err := strconv.ParseFloat(rawValue, 64); err == nil {
				parsedValues[attr.MachineName] = v
			} else {
				parsedValues[attr.MachineName] = rawValue
			}
		default:
			parsedValues[attr.MachineName] = rawValue
		}
	}

	// Transaction
	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		http.Redirect(w, r, "/forms/"+formRefID+"/r/"+recordRefID+"?message=Erro ao iniciar transação", http.StatusSeeOther)
		return
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// Execute pre_save script
	if entityType.PreSave != "" {
		dbAdapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
		dbCtxWithTx := filodb.NewContext(dbAdapter, &txAdapter{tx: tx})

		scriptSetup := func(eng *filo.Engine) {
			filo.RegisterStringBuiltins(eng)
			filodb.RegisterDBBuiltins(eng, dbCtxWithTx)
		}
		modifiedValues, userError, execErr := db.ExecutePreSaveScriptWithSetup(entityType, parsedValues, scriptSetup)
		if execErr != nil {
			http.Redirect(w, r, "/forms/"+formRefID+"/r/"+recordRefID+"?message=Erro no script pre_save: "+execErr.Error(), http.StatusSeeOther)
			return
		}
		if userError != "" {
			http.Redirect(w, r, "/forms/"+formRefID+"/r/"+recordRefID+"?message="+userError, http.StatusSeeOther)
			return
		}
		parsedValues = modifiedValues
	}

	// Update record rev
	err = tx.Exec(`UPDATE eav_records SET rev = rev + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, record.ID)
	if err != nil {
		http.Redirect(w, r, "/forms/"+formRefID+"/r/"+recordRefID+"?message=Erro ao atualizar registro: "+err.Error(), http.StatusSeeOther)
		return
	}

	// Save values
	for machineName, rawValue := range parsedValues {
		var attr *db.EAVAttribute
		for i := range attributes {
			if attributes[i].MachineName == machineName {
				attr = &attributes[i]
				break
			}
		}
		if attr == nil || attr.IsComputed {
			continue
		}

		var vBool, vInt, vReal, vText, vDatetime interface{}
		switch attr.PrimitiveKind {
		case "BOOL":
			if b, ok := rawValue.(bool); ok {
				vBool = b
			}
		case "INT":
			if i, ok := rawValue.(int64); ok {
				vInt = i
			}
		case "REAL":
			if f, ok := rawValue.(float64); ok {
				vReal = f
			}
		case "DATETIME":
			if s, ok := rawValue.(string); ok && s != "" {
				vDatetime = s
			}
		default:
			if s, ok := rawValue.(string); ok {
				vText = s
			}
		}

		err = tx.Exec(`INSERT OR REPLACE INTO eav_values (record_id, attribute_id, v_bool, v_int, v_real, v_text, v_datetime)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			record.ID, attr.ID, vBool, vInt, vReal, vText, vDatetime)
		if err != nil {
			http.Redirect(w, r, "/forms/"+formRefID+"/r/"+recordRefID+"?message=Erro ao salvar valor: "+err.Error(), http.StatusSeeOther)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Redirect(w, r, "/forms/"+formRefID+"/r/"+recordRefID+"?message=Erro ao finalizar: "+err.Error(), http.StatusSeeOther)
		return
	}
	committed = true

	http.Redirect(w, r, "/forms/"+formRefID+"/r/"+recordRefID+"?message=Registro atualizado com sucesso", http.StatusSeeOther)
}

// Unused import placeholder
var _ = sql.ErrNoRows
