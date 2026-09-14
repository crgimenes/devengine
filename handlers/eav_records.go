package handlers

import (
	"encoding/json"
	"maps"
	"net/http"
	"strconv"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/filodb"
	"github.com/crgimenes/devengine/log"
	"github.com/crgimenes/devengine/utils"
	"github.com/crgimenes/filo"
	"github.com/crgimenes/filo/filostrings"
)

// RecordWithValues combines a record with its attribute values
type RecordWithValues struct {
	Record db.EAVRecord
	Values map[string]any // attribute machine_name -> value
}

// ToolsDatabaseSchemaEAVRecords shows list of records with card-based UI
func (h *Handlers) ToolsDatabaseSchemaEAVRecords(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, // check auth
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "ToolsDatabaseSchemaEAVRecords", err)
		return
	}

	if !authed {
		return
	}

	// Sysop-only
	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	// Get entity type ID from path
	entityRefID := r.PathValue("id")
	if entityRefID == "" {
		h.errorPage(w, r, http.StatusBadRequest, "missing entity type ID")
		return
	}

	// Fetch entity type
	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		if err == db.ErrNotFound {
			h.notFound(w, r)
			return
		}
		h.serverError(w, r, "failed to fetch entity type", err)
		return
	}

	// Get all attributes for this entity type
	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		h.serverError(w, r, "failed to fetch attributes", err)
		return
	}

	// Parse offset (default 0)
	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, _ = strconv.Atoi(offsetStr)
	}

	// List records (100 per page, ordered by created_at DESC)
	records, total, err := db.Storage.ListEAVRecordsByEntityTypeID(entityType.ID, 100, offset)
	if err != nil {
		h.serverError(w, r, "failed to fetch records", err)
		return
	}

	// Get values for each record
	recordsWithValues := make([]RecordWithValues, 0, len(records))
	for _, record := range records {
		values, err := db.Storage.GetEAVValuesByRecordID(record.ID)
		if err != nil {
			h.serverError(w, r, "failed to fetch values", err)
			return
		}

		// Convert to map keyed by attribute machine_name
		valueMap := make(map[string]any)
		for _, val := range values {
			// Find attribute
			var attrName string
			for _, attr := range attributes {
				if attr.ID == val.AttributeID {
					attrName = attr.MachineName
					break
				}
			}

			// Get typed value
			switch {
			case val.VBool != nil:
				valueMap[attrName] = *val.VBool
			case val.VInt != nil:
				valueMap[attrName] = *val.VInt
			case val.VReal != nil:
				valueMap[attrName] = *val.VReal
			case val.VText != nil:
				valueMap[attrName] = *val.VText
			case val.VDatetime != nil:
				valueMap[attrName] = *val.VDatetime
			}
		}

		recordsWithValues = append(recordsWithValues, RecordWithValues{
			Record: record,
			Values: valueMap,
		})
	}

	// Calculate hasMore
	hasMore := (offset + 100) < total

	// Get message from query
	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	// Render template
	data := struct {
		Authed      bool
		User        db.User
		Locale      string
		Config      config.Config
		CurrentPage string
		Message     string
		EntityType  *db.EAVEntityType
		Attributes  []db.EAVAttribute
		Records     []RecordWithValues
		Total       int
		Offset      int
		HasMore     bool
	}{
		Authed:      true,
		Locale:      auth.RequestLocale(r),
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "database-schema",
		Message:     message,
		EntityType:  entityType,
		Attributes:  attributes,
		Records:     recordsWithValues,
		Total:       total,
		Offset:      offset,
		HasMore:     hasMore,
	}

	h.render(w, "tools_database_schema_eav_records.go.tmpl", data)
}

// ToolsDatabaseSchemaEAVRecordsAPI returns JSON for infinite scroll pagination
func (h *Handlers) ToolsDatabaseSchemaEAVRecordsAPI(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, // check auth
		true, // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !authed {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !user.Sysop {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	entityRefID := r.PathValue("id")
	if entityRefID == "" {
		http.Error(w, "missing entity type ID", http.StatusBadRequest)
		return
	}

	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		http.Error(w, "entity type not found", http.StatusNotFound)
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to fetch attributes", http.StatusInternalServerError)
		return
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, _ = strconv.Atoi(offsetStr)
	}

	records, total, err := db.Storage.ListEAVRecordsByEntityTypeID(entityType.ID, 100, offset)
	if err != nil {
		http.Error(w, "failed to fetch records", http.StatusInternalServerError)
		return
	}

	// Get values for each record
	recordsWithValues := make([]RecordWithValues, 0, len(records))
	for _, record := range records {
		values, err := db.Storage.GetEAVValuesByRecordID(record.ID)
		if err != nil {
			http.Error(w, "failed to fetch values", http.StatusInternalServerError)
			return
		}

		valueMap := make(map[string]any)
		for _, val := range values {
			var attrName string
			for _, attr := range attributes {
				if attr.ID == val.AttributeID {
					attrName = attr.MachineName
					break
				}
			}

			if val.VBool != nil {
				valueMap[attrName] = *val.VBool
			} else if val.VInt != nil {
				valueMap[attrName] = *val.VInt
			} else if val.VReal != nil {
				valueMap[attrName] = *val.VReal
			} else if val.VText != nil {
				valueMap[attrName] = *val.VText
			} else if val.VDatetime != nil {
				valueMap[attrName] = *val.VDatetime
			}
		}

		recordsWithValues = append(recordsWithValues, RecordWithValues{
			Record: record,
			Values: valueMap,
		})
	}

	hasMore := (offset + 100) < total

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]any{
		"records": recordsWithValues,
		"offset":  offset + 100,
		"hasMore": hasMore,
		"total":   total,
	})
	if err != nil {
		log.Printf("records api encode: %v", err)
	}
}

// ToolsDatabaseSchemaEAVRecordNew shows create form
func (h *Handlers) ToolsDatabaseSchemaEAVRecordNew(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	entityRefID := r.PathValue("id")
	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		h.serverError(w, r, "failed to fetch attributes", err)
		return
	}

	// Prepare initial values with defaults
	values := make(map[string]any)
	for _, attr := range attributes {
		if attr.DefaultVBool != nil {
			// Convert bool pointer to bool value for template
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

	// Execute pos_load script (last step before display)
	var posLoadError string
	if entityType.PosLoad != "" {
		modifiedValues, userError, execErr := db.ExecutePosLoadScript(entityType, db.EAVRecordValues(values))
		if execErr != nil {
			// Log error but continue with original values
			// Script errors shouldn't block viewing
		} else {
			// Update values with modified values
			maps.Copy(values, modifiedValues)
			posLoadError = userError
		}
	}

	data := struct {
		Authed       bool
		User         db.User
		Locale       string
		Config       config.Config
		CurrentPage  string
		EntityType   *db.EAVEntityType
		Attributes   []db.EAVAttribute
		Record       *db.EAVRecord
		Values       map[string]any
		Message      string
		PosLoadError string
	}{
		Authed:       true,
		Locale:       auth.RequestLocale(r),
		User:         *user,
		Config:       *h.cfg,
		CurrentPage:  "database-schema",
		EntityType:   entityType,
		Attributes:   attributes,
		Record:       nil, // New record
		Values:       values,
		Message:      r.URL.Query().Get("message"),
		PosLoadError: posLoadError,
	}

	h.render(w, "tools_database_schema_eav_record_edit.go.tmpl", data)
}

// parseAdminRecordValues extracts typed values from the admin record form.
// On a validation problem it returns the user-facing message instead.
func parseAdminRecordValues(r *http.Request, attributes []db.EAVAttribute) (db.EAVRecordValues, string) {
	parsed := make(db.EAVRecordValues)
	for _, attr := range attributes {
		value := r.FormValue("attr_" + attr.MachineName)

		if value == "" && attr.IsRequired {
			return nil, tr(r, "Required field: %s", attr.Label)
		}
		// Keep empty optionals visible to pre_save scripts.
		if value == "" {
			parsed[attr.MachineName] = ""
			continue
		}

		switch attr.PrimitiveKind {
		case "BOOL":
			parsed[attr.MachineName] = value == "1" || value == "true"
		case "INT":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, tr(r, "Invalid value for %s", attr.Label)
			}
			parsed[attr.MachineName] = intVal
		case "REAL":
			realVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, tr(r, "Invalid value for %s", attr.Label)
			}
			parsed[attr.MachineName] = realVal
		case "TEXT":
			if attr.MaxLength != nil && len(value) > *attr.MaxLength {
				return nil, tr(r, "Field %s exceeds the limit of %d characters", attr.Label, *attr.MaxLength)
			}
			parsed[attr.MachineName] = value
		case "DATETIME":
			parsed[attr.MachineName] = value
		}
	}
	return parsed, ""
}

// typedEAVValue converts a parsed value back to the typed pointer set used by
// the eav_values columns. All pointers nil means "no value" (empty optional).
func typedEAVValue(kind string, raw any) (vBool *bool, vInt *int64, vReal *float64, vText, vDatetime *string) {
	switch kind {
	case "BOOL":
		v, ok := raw.(bool)
		if ok {
			vBool = &v
		}
	case "INT":
		v, ok := raw.(int64)
		if ok {
			vInt = &v
		}
	case "REAL":
		v, ok := raw.(float64)
		if ok {
			vReal = &v
			return
		}
		// Scripts may hand back an int where a REAL is expected.
		i, ok := raw.(int64)
		if ok {
			f := float64(i)
			vReal = &f
		}
	case "TEXT":
		v, ok := raw.(string)
		if ok {
			vText = &v
		}
	case "DATETIME":
		v, ok := raw.(string)
		if ok {
			vDatetime = &v
		}
	}
	return
}

// eavUniquePointer returns the pointer matching the attribute's kind for the
// uniqueness check, or nil when the value is empty.
func eavUniquePointer(kind string, vBool *bool, vInt *int64, vReal *float64, vText, vDatetime *string) any {
	switch kind {
	case "BOOL":
		if vBool != nil {
			return vBool
		}
	case "INT":
		if vInt != nil {
			return vInt
		}
	case "REAL":
		if vReal != nil {
			return vReal
		}
	case "TEXT":
		if vText != nil {
			return vText
		}
	case "DATETIME":
		if vDatetime != nil {
			return vDatetime
		}
	}
	return nil
}

// runAdminPreSave executes the entity's pre_save script inside the given
// transaction context. It returns the (possibly modified) values, or the
// user-facing message when the script blocks or fails.
func runAdminPreSave(r *http.Request, entityType *db.EAVEntityType, tx db.Tx, values db.EAVRecordValues) (db.EAVRecordValues, string) {
	if entityType.PreSave == "" {
		return values, ""
	}
	dbAdapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
	dbCtxWithTx := filodb.NewContext(dbAdapter, tx)
	scriptSetup := func(eng *filo.Engine) {
		filostrings.RegisterBuiltins(eng)
		filodb.RegisterDBBuiltins(eng, dbCtxWithTx)
	}
	modified, userError, execErr := db.ExecutePreSaveScriptWithSetup(entityType, values, scriptSetup)
	if execErr != nil {
		return nil, tr(r, "Script error: %s", execErr.Error())
	}
	if userError != "" {
		return nil, userError
	}
	return modified, ""
}

// ToolsDatabaseSchemaEAVRecordCreate creates a new record.
func (h *Handlers) ToolsDatabaseSchemaEAVRecordCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	entityRefID := r.PathValue("id")
	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		h.serverError(w, r, "failed to fetch attributes", err)
		return
	}

	redirectBack := func(message string) {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message="+message, http.StatusSeeOther)
	}

	parsedValues, msg := parseAdminRecordValues(r, attributes)
	if msg != "" {
		redirectBack(msg)
		return
	}

	// pre_save script and EAV save share the same transaction.
	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		redirectBack(tr(r, "Could not start the transaction"))
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	parsedValues, msg = runAdminPreSave(r, entityType, tx, parsedValues)
	if msg != "" {
		redirectBack(msg)
		return
	}

	refID := utils.NewOpaqueID()
	recordID, err := tx.InsertEAVRecordWithRef(refID, entityType.ID, "draft")
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message="+tr(r, "Could not create the record"), http.StatusSeeOther)
		return
	}

	attrByMachine := make(map[string]db.EAVAttribute, len(attributes))
	for _, attr := range attributes {
		attrByMachine[attr.MachineName] = attr
	}

	for machineName, rawValue := range parsedValues {
		attr, ok := attrByMachine[machineName]
		if !ok {
			continue // skip values for unknown attributes
		}

		vBool, vInt, vReal, vText, vDatetime := typedEAVValue(attr.PrimitiveKind, rawValue)
		// An empty optional has no typed value; storing an all-NULL row would
		// violate the one-value CHECK on eav_values.
		if vBool == nil && vInt == nil && vReal == nil && vText == nil && vDatetime == nil {
			continue
		}

		uniqueValue := eavUniquePointer(attr.PrimitiveKind, vBool, vInt, vReal, vText, vDatetime)
		if attr.IsUnique && uniqueValue != nil {
			isUnique, err := db.Storage.CheckEAVValueUnique(attr.ID, attr.PrimitiveKind, uniqueValue, 0)
			if err != nil {
				ref := logRef("ToolsDatabaseSchemaEAVRecordCreate", err)
				redirectBack(tr(r, "Could not validate uniqueness (ref %s)", ref))
				return
			}
			if !isUnique {
				redirectBack(tr(r, "Value already exists for field %s", attr.Label))
				return
			}
		}

		if err := tx.UpsertEAVValue(recordID, attr.ID, vBool, vInt, vReal, vText, vDatetime); err != nil {
			ref := logRef("ToolsDatabaseSchemaEAVRecordCreate", err)
			redirectBack(tr(r, "Could not save the value (ref %s)", ref))
			return
		}
	}

	// Activation must bump rev to satisfy the trigger constraint.
	err = tx.ActivateEAVRecord(recordID)
	if err != nil {
		ref := logRef("ToolsDatabaseSchemaEAVRecordCreate", err)
		redirectBack(tr(r, "Could not activate the record (ref %s)", ref))
		return
	}

	err = tx.Commit()
	if err != nil {
		ref := logRef("ToolsDatabaseSchemaEAVRecordCreate", err)
		redirectBack(tr(r, "Could not save (ref %s)", ref))
		return
	}
	committed = true

	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message="+tr(r, "Record created successfully"), http.StatusSeeOther)
}

// ToolsDatabaseSchemaEAVRecordEdit shows edit form
func (h *Handlers) ToolsDatabaseSchemaEAVRecordEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	entityRefID := r.PathValue("id")
	recordRefID := r.PathValue("record_id")

	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	record, err := db.Storage.GetEAVRecordByRefID(recordRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	if record.EntityTypeID != entityType.ID {
		h.errorPage(w, r, http.StatusBadRequest, "Record does not belong to this entity type")
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		h.serverError(w, r, "failed to fetch attributes", err)
		return
	}

	values, err := db.Storage.GetEAVValuesByRecordID(record.ID)
	if err != nil {
		h.serverError(w, r, "failed to fetch values", err)
		return
	}

	// Convert to map
	valueMap := make(map[string]any)
	for _, val := range values {
		var attrName string
		for _, attr := range attributes {
			if attr.ID == val.AttributeID {
				attrName = attr.MachineName
				break
			}
		}

		if val.VBool != nil {
			valueMap[attrName] = *val.VBool
		} else if val.VInt != nil {
			valueMap[attrName] = *val.VInt
		} else if val.VReal != nil {
			valueMap[attrName] = *val.VReal
		} else if val.VText != nil {
			valueMap[attrName] = *val.VText
		} else if val.VDatetime != nil {
			valueMap[attrName] = *val.VDatetime
		}
	}

	// Apply default values for missing fields
	for _, attr := range attributes {
		if _, exists := valueMap[attr.MachineName]; !exists {
			if attr.DefaultVBool != nil {
				valueMap[attr.MachineName] = *attr.DefaultVBool
			} else if attr.DefaultVInt != nil {
				valueMap[attr.MachineName] = *attr.DefaultVInt
			} else if attr.DefaultVReal != nil {
				valueMap[attr.MachineName] = *attr.DefaultVReal
			} else if attr.DefaultVText != nil {
				valueMap[attr.MachineName] = *attr.DefaultVText
			} else if attr.DefaultVDatetime != nil {
				valueMap[attr.MachineName] = datetimeDefault(*attr.DefaultVDatetime)
			}
		}
	}

	// Execute pos_load script (last step before display)
	var posLoadError string
	if entityType.PosLoad != "" {
		modifiedValues, userError, execErr := db.ExecutePosLoadScript(entityType, db.EAVRecordValues(valueMap))
		if execErr != nil {
			// Log error but continue with original values
			// Script errors shouldn't block viewing
		} else {
			// Update valueMap with modified values
			maps.Copy(valueMap, modifiedValues)
			posLoadError = userError
		}
	}

	// Get message from query
	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed       bool
		User         db.User
		Locale       string
		Config       config.Config
		CurrentPage  string
		EntityType   *db.EAVEntityType
		Attributes   []db.EAVAttribute
		Record       *db.EAVRecord
		Values       map[string]any
		Message      string
		PosLoadError string
	}{
		Authed:       true,
		Locale:       auth.RequestLocale(r),
		User:         *user,
		Config:       *h.cfg,
		CurrentPage:  "database-schema",
		EntityType:   entityType,
		Attributes:   attributes,
		Record:       record,
		Values:       valueMap,
		Message:      message,
		PosLoadError: posLoadError,
	}

	h.render(w, "tools_database_schema_eav_record_edit.go.tmpl", data)
}

// ToolsDatabaseSchemaEAVRecordUpdate updates an existing record.
func (h *Handlers) ToolsDatabaseSchemaEAVRecordUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	entityRefID := r.PathValue("id")
	recordRefID := r.PathValue("record_id")

	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	record, err := db.Storage.GetEAVRecordByRefID(recordRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}
	if record.EntityTypeID != entityType.ID {
		h.errorPage(w, r, http.StatusBadRequest, "Record does not belong to this entity type")
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		h.serverError(w, r, "failed to fetch attributes", err)
		return
	}

	redirectBack := func(message string) {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message="+message, http.StatusSeeOther)
	}

	currentRev, err := strconv.Atoi(r.FormValue("rev"))
	if err != nil {
		redirectBack(tr(r, "Error: invalid revision"))
		return
	}

	parsedValues, msg := parseAdminRecordValues(r, attributes)
	if msg != "" {
		redirectBack(msg)
		return
	}

	// pre_save runs in its own transaction; the value upserts below rely on
	// the per-value optimistic locking instead.
	if entityType.PreSave != "" {
		tx, err := db.Storage.BeginTransaction()
		if err != nil {
			redirectBack(tr(r, "Could not start the transaction"))
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		parsedValues, msg = runAdminPreSave(r, entityType, tx, parsedValues)
		if msg != "" {
			redirectBack(msg)
			return
		}
		err = tx.Commit()
		if err != nil {
			ref := logRef("ToolsDatabaseSchemaEAVRecordUpdate", err)
			redirectBack(tr(r, "Could not save the script (ref %s)", ref))
			return
		}
		committed = true
	}

	attrByMachine := make(map[string]db.EAVAttribute, len(attributes))
	for _, attr := range attributes {
		attrByMachine[attr.MachineName] = attr
	}

	for machineName, rawValue := range parsedValues {
		attr, ok := attrByMachine[machineName]
		if !ok {
			continue // skip values for unknown attributes
		}

		vBool, vInt, vReal, vText, vDatetime := typedEAVValue(attr.PrimitiveKind, rawValue)
		// An empty optional has no typed value; the rev-checked upsert refuses
		// an all-NULL row, so keep whatever is stored.
		if vBool == nil && vInt == nil && vReal == nil && vText == nil && vDatetime == nil {
			continue
		}

		uniqueValue := eavUniquePointer(attr.PrimitiveKind, vBool, vInt, vReal, vText, vDatetime)
		if attr.IsUnique && uniqueValue != nil {
			isUnique, err := db.Storage.CheckEAVValueUnique(attr.ID, attr.PrimitiveKind, uniqueValue, record.ID)
			if err != nil {
				ref := logRef("ToolsDatabaseSchemaEAVRecordUpdate", err)
				redirectBack(tr(r, "Could not validate uniqueness (ref %s)", ref))
				return
			}
			if !isUnique {
				redirectBack(tr(r, "Value already exists for field %s", attr.Label))
				return
			}
		}

		_, err = db.Storage.UpsertEAVValueWithRev(record.ID, attr.ID, currentRev, vBool, vInt, vReal, vText, vDatetime)
		if err != nil {
			if err == db.ErrConflict {
				redirectBack(tr(r, "Conflict: the record was modified by another user. Reload the page."))
				return
			}
			ref := logRef("ToolsDatabaseSchemaEAVRecordUpdate", err)
			redirectBack(tr(r, "Could not update (ref %s)", ref))
			return
		}

		// Each rev-checked upsert bumps the record rev.
		currentRev++
	}

	err = db.Storage.UpdateEAVRecordStatus(record.ID, currentRev, "active")
	if err != nil {
		ref := logRef("ToolsDatabaseSchemaEAVRecordUpdate", err)
		redirectBack(tr(r, "Could not activate the record (ref %s)", ref))
		return
	}

	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message="+tr(r, "Record updated successfully"), http.StatusSeeOther)
}

// ToolsDatabaseSchemaEAVRecordDelete soft deletes a record
func (h *Handlers) ToolsDatabaseSchemaEAVRecordDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	entityRefID := r.PathValue("id")
	recordRefID := r.PathValue("record_id")

	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	record, err := db.Storage.GetEAVRecordByRefID(recordRefID)
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message="+tr(r, "Record not found"), http.StatusSeeOther)
		return
	}
	if record.EntityTypeID != entityType.ID {
		h.errorPage(w, r, http.StatusBadRequest, "Record does not belong to this entity type")
		return
	}

	err = db.Storage.SoftDeleteEAVRecord(record.ID)
	if err != nil {
		ref := logRef("ToolsDatabaseSchemaEAVRecordDelete", err)
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message="+tr(r, "Could not delete record (ref %s)", ref), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message="+tr(r, "Record deleted successfully"), http.StatusSeeOther)
}
