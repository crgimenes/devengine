package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/crgimenes/devengine/log"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/eav/ui"
	"github.com/crgimenes/devengine/filodb"
	"github.com/crgimenes/devengine/filoeav"
	"github.com/crgimenes/devengine/filofile"
	"github.com/crgimenes/devengine/filolog"
	"github.com/crgimenes/devengine/filosession"
	"github.com/crgimenes/devengine/i18n"
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
	Element    db.FormElement
	Attribute  *db.EAVAttribute
	Children   []FormRuntimeNode
	FieldError string // per-field validation message for the re-rendered form
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
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "FormsRuntimeNew", err)
		return
	}
	if !authed {
		return
	}

	ctx, ok := h.loadRuntimeForm(w, r, false)
	if !ok {
		return
	}

	// A search form's entry point is its listing (text search + rows), not
	// the create view.
	if ctx.form.IsSearch {
		h.FormsRuntimeList(w, r)
		return
	}

	// Initial values come from the attribute defaults.
	values := make(map[string]any)
	for _, attr := range ctx.attributes {
		if attr.DefaultVBool != nil {
			values[attr.MachineName] = *attr.DefaultVBool
		} else if attr.DefaultVInt != nil {
			values[attr.MachineName] = *attr.DefaultVInt
		} else if attr.DefaultVReal != nil {
			values[attr.MachineName] = *attr.DefaultVReal
		} else if attr.DefaultVText != nil {
			values[attr.MachineName] = *attr.DefaultVText
		} else if attr.DefaultVDatetime != nil {
			values[attr.MachineName] = datetimeDefault(*attr.DefaultVDatetime)
		}
	}

	// Prefill from ?prefill=attr=value (may repeat). Used by the subform's
	// "Novo" link to pre-populate the reference back to the parent record.
	applyPrefill(values, ctx.attributes, r.URL.Query()["prefill"])

	var posLoadError string
	if ctx.entityType != nil && ctx.entityType.PosLoad != "" {
		modifiedValues, userError, execErr := db.ExecutePosLoadScript(ctx.entityType, db.EAVRecordValues(values))
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

	h.renderRuntimeForm(w, user, ctx, nil, values, nil, message, errorMsg, posLoadError)
}

// FormsRuntimeCreate handles form submission to create a new record. On a
// validation problem it re-renders the form with what the user typed and the
// message next to each offending field.
func (h *Handlers) FormsRuntimeCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed {
		h.forbidden(w, r)
		return
	}

	ctx, ok := h.loadRuntimeForm(w, r, true)
	if !ok {
		return
	}
	machineName := ctx.form.MachineName

	parsedValues, fieldErrors := parseFormAttributesLenient(r, ctx.elements, ctx.attributes)
	if len(fieldErrors) > 0 {
		h.renderRuntimeForm(w, user, ctx, nil, parsedValues, fieldErrors, "", "", "")
		return
	}

	parsedValues, err = applyComputedExprs(r.Context(), user, ctx.attributes, parsedValues)
	if err != nil {
		h.serverError(w, r, "FormsRuntimeCreate", err)
		return
	}

	fieldErrors, sysErr := evaluateValidateExprs(r.Context(), user, ctx.elements, ctx.attributes, parsedValues)
	if sysErr != nil {
		h.serverError(w, r, "FormsRuntimeCreate", err)
		return
	}
	if len(fieldErrors) > 0 {
		h.renderRuntimeForm(w, user, ctx, nil, parsedValues, fieldErrors, "", "", "")
		return
	}

	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		h.renderRuntimeForm(w, user, ctx, nil, parsedValues, nil, "", i18n.T("Could not start the transaction"), "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Helper handles pre_save, record creation and value saving. Failures
	// (pre_save block, unique violation) re-render keeping the typed values.
	_, recordRefID, err := insertRecordTx(tx, ctx.entityType, ctx.attributes, parsedValues)
	if err != nil {
		h.renderRuntimeForm(w, user, ctx, nil, parsedValues, nil, "", err.Error(), "")
		return
	}

	err = tx.Commit()
	if err != nil {
		ref := logRef("create commit", err)
		h.renderRuntimeForm(w, user, ctx, nil, parsedValues, nil, "", i18n.T("Could not finish saving (ref %s)", ref), "")
		return
	}
	committed = true

	http.Redirect(w, r, "/form/"+machineName+"/r/"+recordRefID+"?message="+i18n.T("Record created successfully"), http.StatusSeeOther)
}

// FormsRuntimeEdit shows a form for editing an existing record.
// datetimeDefault resolves the special default "now" to the current
// timestamp in the HTML5 datetime-local layout; other values pass through.
func datetimeDefault(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "now") {
		return time.Now().Format("2006-01-02T15:04")
	}
	return v
}

// recordValuesMap flattens typed EAV values into machine_name → Go value.
func recordValuesMap(eavValues []db.EAVValue, attributes []db.EAVAttribute) map[string]any {
	attrByID := make(map[int64]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		attrByID[attributes[i].ID] = &attributes[i]
	}
	values := make(map[string]any, len(eavValues))
	for _, val := range eavValues {
		attr := attrByID[val.AttributeID]
		if attr == nil {
			continue
		}
		v := unwrapValue(attr.PrimitiveKind, val)
		if v != nil {
			values[attr.MachineName] = v
		}
	}
	return values
}

func (h *Handlers) FormsRuntimeEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "FormsRuntimeEdit", err)
		return
	}
	if !authed {
		return
	}

	ctx, ok := h.loadRuntimeForm(w, r, true)
	if !ok {
		return
	}

	record, err := db.Storage.GetEAVRecordByRefID(r.PathValue("recordRef"))
	if err != nil {
		h.notFound(w, r)
		return
	}
	if record.EntityTypeID != ctx.entityType.ID {
		h.errorPage(w, r, http.StatusBadRequest, "Record does not belong to this form's entity type")
		return
	}

	eavValues, err := db.Storage.GetEAVValuesByRecordID(record.ID)
	if err != nil {
		h.serverError(w, r, "failed to list values", err)
		return
	}
	values := recordValuesMap(eavValues, ctx.attributes)

	var posLoadError string
	if ctx.entityType.PosLoad != "" {
		modifiedValues, userError, execErr := db.ExecutePosLoadScript(ctx.entityType, db.EAVRecordValues(values))
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

	h.renderRuntimeForm(w, user, ctx, record, values, nil, message, errorMsg, posLoadError)
}

// FormsRuntimeUpdate handles form submission to update an existing record.
// Validation problems re-render the edit view keeping the typed values.
func (h *Handlers) FormsRuntimeUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed {
		h.forbidden(w, r)
		return
	}

	ctx, ok := h.loadRuntimeForm(w, r, true)
	if !ok {
		return
	}
	machineName := ctx.form.MachineName
	recordRefID := r.PathValue("recordRef")

	record, err := db.Storage.GetEAVRecordByRefID(recordRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	parsedValues, fieldErrors := parseFormAttributesLenient(r, ctx.elements, ctx.attributes)

	// Optimistic locking: the hidden rev must match the current record.
	submittedRev, _ := strconv.Atoi(r.FormValue("rev"))
	if submittedRev != record.Rev {
		h.renderRuntimeForm(w, user, ctx, record, parsedValues, nil, "",
			i18n.T("The record was modified by another user. Review the data before saving again."), "")
		return
	}

	if len(fieldErrors) > 0 {
		h.renderRuntimeForm(w, user, ctx, record, parsedValues, fieldErrors, "", "", "")
		return
	}

	parsedValues, err = applyComputedExprs(r.Context(), user, ctx.attributes, parsedValues)
	if err != nil {
		h.serverError(w, r, "FormsRuntimeUpdate", err)
		return
	}

	fieldErrors, sysErr := evaluateValidateExprs(r.Context(), user, ctx.elements, ctx.attributes, parsedValues)
	if sysErr != nil {
		h.serverError(w, r, "FormsRuntimeUpdate", err)
		return
	}
	if len(fieldErrors) > 0 {
		h.renderRuntimeForm(w, user, ctx, record, parsedValues, fieldErrors, "", "", "")
		return
	}

	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		h.renderRuntimeForm(w, user, ctx, record, parsedValues, nil, "", i18n.T("Could not start the transaction"), "")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Helper handles pre_save, revision update and value saving.
	err = updateRecordTx(tx, ctx.entityType, record, ctx.attributes, parsedValues)
	if err != nil {
		h.renderRuntimeForm(w, user, ctx, record, parsedValues, nil, "", err.Error(), "")
		return
	}

	err = tx.Commit()
	if err != nil {
		ref := logRef("update commit", err)
		h.renderRuntimeForm(w, user, ctx, record, parsedValues, nil, "", i18n.T("Could not finish saving (ref %s)", ref), "")
		return
	}
	committed = true

	http.Redirect(w, r, "/form/"+machineName+"/r/"+recordRefID+"?message="+i18n.T("Record updated successfully"), http.StatusSeeOther)
}

// FormsRuntimeButtonAction handles custom button actions.
// Executes save action (if configured) and Filo code in the same transaction.
func (h *Handlers) FormsRuntimeButtonAction(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil {
		ref := logRef("FormsRuntimeButtonAction prelude", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "erro interno (ref " + ref + ")"})
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
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil { // #nosec G120 -- bounded by MaxBytesReader above
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

	parsedValues := make(db.EAVRecordValues)
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
			_ = tx.Rollback()
		}
	}()

	// =========================================================
	// 2. Execute Filo Script (BEFORE Save)
	// =========================================================
	if button.ButtonFiloCode != "" {
		globals := buttonFiloGlobals(form, user, currentRecord, parsedValues)
		newGlobals, execErr := runButtonFilo(r.Context(), tx, user, button.ButtonFiloCode, globals)
		if execErr != nil {
			log.Printf("[ERROR] Button '%s' Filo script error: %v", button.MachineName, execErr)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Script error: " + execErr.Error()})
			return
		}
		applyButtonGlobals(newGlobals, attributes, parsedValues, response)
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
		parsedValues, err = applyComputedExprs(r.Context(), user, attributes, parsedValues)
		if err != nil {
			log.Printf("[ERROR] button computed_expr: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "computed_expr failed"})
			return
		}
		fieldErrors, sysErr := evaluateValidateExprs(r.Context(), user, elements, attributes, parsedValues)
		if sysErr != nil {
			log.Printf("[ERROR] button validate_expr: %v", sysErr)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "validate_expr failed"})
			return
		}
		if len(fieldErrors) > 0 {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": joinFieldErrors(elements, fieldErrors)})
			return
		}

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
			response["redirect_to"] = "/form/" + machineName + "/r/" + newRefID
		}
	}

	// 4. Commit Transaction
	if err := tx.Commit(); err != nil {
		ref := logRef("button commit", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": i18n.T("Could not finish saving (ref %s)", ref)})
		return
	}
	committed = true

	jsonResponse(w, http.StatusOK, response)
}

// buttonFiloGlobals assembles the global scope handed to a button's Filo
// script: field values, form/user/record metadata and the control variables.
func buttonFiloGlobals(form *db.Form, user *db.User, currentRecord *db.EAVRecord, parsedValues db.EAVRecordValues) map[string]filo.Value {
	globals := make(map[string]filo.Value)

	for k, v := range parsedValues {
		globals["field:"+k] = goToFilo(v)
	}

	globals["form:machine_name"] = filo.VString(form.MachineName)
	globals["form:label"] = filo.VString(form.Label)
	globals["form:reference_id"] = filo.VString(form.ReferenceID)

	if user != nil {
		globals["user:id"] = filo.VNum(float64(user.ID))
		globals["user:email"] = filo.VString(user.Email)
		globals["user:sysop"] = filo.VBool(user.Sysop)
	}

	globals["record:id"] = filo.VNum(0)
	globals["record:ref_id"] = filo.VString("")
	if currentRecord != nil {
		globals["record:id"] = filo.VNum(float64(currentRecord.ID))
		globals["record:ref_id"] = filo.VString(currentRecord.ReferenceID)
		globals["record:status"] = filo.VString(currentRecord.Status)
	}

	// Control variables the script may set to influence the response.
	globals["error"] = filo.VString("")
	globals["message"] = filo.VString("")
	globals["redirect_to"] = filo.VString("")

	return globals
}

// runButtonFilo executes the script with the engine wired to the SAME
// transaction, so script-issued DB ops roll back together with the save.
// Button scripts get the full builtin set of the other form contexts plus
// filodb for raw statements.
func runButtonFilo(ctx context.Context, tx *db.Transaction, user *db.User, code string, globals map[string]filo.Value) (map[string]filo.Value, error) {
	eng := filo.NewEngine()

	dbAdapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
	dbCtxWithTx := filodb.NewContext(dbAdapter, &txAdapter{tx: tx})

	filostrings.RegisterBuiltins(eng)
	filodb.RegisterDBBuiltins(eng, dbCtxWithTx)
	filoeav.RegisterEAVBuiltins(eng, filoeav.NewContextTx(db.Storage, &txAdapter{tx: tx}))
	filofile.RegisterFileBuiltins(eng, filofile.NewContext(db.Storage))
	filosession.RegisterSessionBuiltins(eng, filosession.NewContext(user))
	filolog.RegisterLogBuiltins(eng, filolog.NewContext(globals))

	cfg := filo.EvalConfig{
		StepLimit:      10000,
		RecursionLimit: 100,
		Timeout:        5 * 1e9, // 5 seconds in nanoseconds
	}

	_, newGlobals, err := eng.RunScript(ctx, code, globals, cfg)
	return newGlobals, err
}

// applyButtonGlobals maps the script's resulting globals back onto the parsed
// values (typed per attribute, so a whole-number Filo value still lands as
// REAL when the attribute says so) and the response control variables.
func applyButtonGlobals(newGlobals map[string]filo.Value, attributes []db.EAVAttribute, parsedValues db.EAVRecordValues, response map[string]any) {
	attrByName := make(map[string]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		attrByName[attributes[i].MachineName] = &attributes[i]
	}

	for kRaw, val := range newGlobals {
		if kRaw == "error" && val.Kind == filo.KString && val.Str != "" {
			response["error"] = val.Str
		}
		if kRaw == "message" && val.Kind == filo.KString && val.Str != "" {
			response["message"] = val.Str
		}
		if kRaw == "redirect_to" && val.Kind == filo.KString && val.Str != "" {
			response["redirect_to"] = val.Str
		}

		switch val.Kind {
		case filo.KNumber, filo.KString, filo.KBool:
		default:
			continue
		}

		// Unprefixed globals only map back when they match a known attribute,
		// so control names like "error" cannot collide.
		k := kRaw
		if after, ok := strings.CutPrefix(k, "field:"); ok {
			k = after
		}
		attr := attrByName[k]
		if attr == nil {
			continue
		}
		parsedValues[k] = filoToGoTyped(attr.PrimitiveKind, val)
	}
}

// jsonResponse writes a JSON response with the given status code.
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	err := enc.Encode(data)
	if err != nil {
		log.Printf("jsonResponse encode: %v", err)
	}
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

	_, _ = w.Write([]byte(js.String()))
}

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
				return nil, fmt.Errorf("%s", i18n.T("required field: %s", attr.Label))
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

// defaultUIKind maps a primitive kind to the field plugin used when an element
// does not pin an explicit ui_kind. It mirrors the fallback in the runtime
// template so saving and rendering agree on which plugin owns a value.
func defaultUIKind(primitiveKind string) string {
	switch primitiveKind {
	case "BOOL":
		return "bool"
	case "INT":
		return "int"
	case "REAL":
		return "decimal"
	case "DATETIME":
		return "datetime"
	default:
		return "text"
	}
}

// parseElementValue prefers a registered ui.FieldUI for the element's UIKind
// and falls back to the primitive-kind switch when no plugin is registered.
// This is the only interpretation point of raw form values during create/update.
func parseElementValue(el db.FormElement, attr *db.EAVAttribute, raw string) (any, error) {
	uiKind := el.UIKind
	if uiKind == "" {
		uiKind = defaultUIKind(attr.PrimitiveKind)
	}
	plugin, ok := ui.Get(uiKind)
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
		dbCtxWithTx := filodb.NewContext(dbAdapter, &txAdapter{tx: tx})

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
	var recordID int64
	var recordRefID string
	err := tx.QueryRow(`INSERT INTO eav_records (reference_id, entity_type_id, status, rev) VALUES (?, ?, 'active', 1) RETURNING id, reference_id`,
		refID, entityType.ID).Scan(&recordID, &recordRefID)
	if err != nil {
		return 0, "", fmt.Errorf("erro ao criar registro: %w", err)
	}

	// Save values
	if err := saveValuesTx(tx, recordID, attributes, values); err != nil {
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
		dbCtxWithTx := filodb.NewContext(dbAdapter, &txAdapter{tx: tx})

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
	err := tx.Exec(`UPDATE eav_records SET rev = rev + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, record.ID)
	if err != nil {
		return fmt.Errorf("erro ao atualizar registro: %w", err)
	}

	// Save values
	return saveValuesTx(tx, record.ID, attributes, values)
}

// saveValuesTx inserts or replaces values for a record within a transaction.
func saveValuesTx(tx *db.Transaction, recordID int64, attributes []db.EAVAttribute, values db.EAVRecordValues) error {
	for machineName, rawValue := range values {
		var attr *db.EAVAttribute
		for i := range attributes {
			if attributes[i].MachineName == machineName {
				attr = &attributes[i]
				break
			}
		}
		if attr == nil {
			continue
		}
		// Computed values persist too: applyComputedExprs already overwrote
		// any user input upstream, and lists/CSV read straight from eav_values.

		var vBool, vInt, vReal, vText, vDatetime any
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
			} else if i, ok := rawValue.(int64); ok {
				vReal = float64(i)
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

		// No value set means the field is empty (e.g. an optional datetime).
		// Storing a row with every column NULL violates the one-value CHECK, so
		// drop any prior value and skip the insert.
		if vBool == nil && vInt == nil && vReal == nil && vText == nil && vDatetime == nil {
			err := tx.Exec(`DELETE FROM eav_values WHERE record_id = ? AND attribute_id = ?`,
				recordID, attr.ID)
			if err != nil {
				return fmt.Errorf("erro ao limpar valor: %w", err)
			}
			continue
		}

		err := tx.Exec(`INSERT OR REPLACE INTO eav_values (record_id, attribute_id, v_bool, v_int, v_real, v_text, v_datetime)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			recordID, attr.ID, vBool, vInt, vReal, vText, vDatetime)
		if err != nil {
			return fmt.Errorf("erro ao salvar valor: %w", err)
		}
	}
	return nil
}
