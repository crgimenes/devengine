package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/filodb"
	"github.com/crgimenes/devengine/utils"
	"github.com/crgimenes/filo"
)

// RecordWithValues combines a record with its attribute values
type RecordWithValues struct {
	Record db.EAVRecord
	Values map[string]interface{} // attribute machine_name -> value
}

// txAdapter adapts *db.Transaction to filodb.DBTransaction interface
type txAdapter struct {
	tx *db.Transaction
}

func (a *txAdapter) Query(query string, args ...any) (*sql.Rows, error) {
	return a.tx.Query(query, args...)
}

func (a *txAdapter) Exec(query string, args ...any) error {
	return a.tx.Exec(query, args...)
}

func (a *txAdapter) Commit() error {
	return a.tx.Commit()
}

func (a *txAdapter) Rollback() error {
	return a.tx.Rollback()
}

// ToolsDatabaseSchemaEAVRecords shows list of records with card-based UI
func (h *Handlers) ToolsDatabaseSchemaEAVRecords(w http.ResponseWriter, r *http.Request) {
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

	// Sysop-only
	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get entity type ID from path
	entityRefID := r.PathValue("id")
	if entityRefID == "" {
		http.Error(w, "missing entity type ID", http.StatusBadRequest)
		return
	}

	// Fetch entity type
	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		if err == db.ErrNotFound {
			http.Error(w, "Entity type not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch entity type", http.StatusInternalServerError)
		return
	}

	// Get all attributes for this entity type
	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to fetch attributes", http.StatusInternalServerError)
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

		// Convert to map keyed by attribute machine_name
		valueMap := make(map[string]interface{})
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

	err = h.templates(w, "tools_database_schema_eav_records.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// ToolsDatabaseSchemaEAVRecordsAPI returns JSON for infinite scroll pagination
func (h *Handlers) ToolsDatabaseSchemaEAVRecordsAPI(w http.ResponseWriter, r *http.Request) {
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

		valueMap := make(map[string]interface{})
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"records": recordsWithValues,
		"offset":  offset + 100,
		"hasMore": hasMore,
		"total":   total,
	})
}

// ToolsDatabaseSchemaEAVRecordNew shows create form
func (h *Handlers) ToolsDatabaseSchemaEAVRecordNew(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	entityRefID := r.PathValue("id")
	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		http.Error(w, "Entity type not found", http.StatusNotFound)
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to fetch attributes", http.StatusInternalServerError)
		return
	}

	// Prepare initial values with defaults
	values := make(map[string]interface{})
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
			values[attr.MachineName] = *attr.DefaultVDatetime
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
			for k, v := range modifiedValues {
				values[k] = v
			}
			posLoadError = userError
		}
	}

	data := struct {
		Authed       bool
		User         db.User
		Config       config.Config
		CurrentPage  string
		EntityType   *db.EAVEntityType
		Attributes   []db.EAVAttribute
		Record       *db.EAVRecord
		Values       map[string]interface{}
		Message      string
		PosLoadError string
	}{
		Authed:       true,
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

	err = h.templates(w, "tools_database_schema_eav_record_edit.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// ToolsDatabaseSchemaEAVRecordCreate creates new record
func (h *Handlers) ToolsDatabaseSchemaEAVRecordCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	entityRefID := r.PathValue("id")
	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		http.Error(w, "Entity type not found", http.StatusNotFound)
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to fetch attributes", http.StatusInternalServerError)
		return
	}

	// Build attribute lookup by machine_name for later use
	attrByMachine := make(map[string]db.EAVAttribute)
	for _, attr := range attributes {
		attrByMachine[attr.MachineName] = attr
	}

	// =========================================================
	// PHASE 1: Parse all values into a map (without saving)
	// =========================================================
	parsedValues := make(db.EAVRecordValues)

	for _, attr := range attributes {
		value := r.FormValue("attr_" + attr.MachineName)

		// For empty non-required fields, still set the variable so scripts can check it
		if value == "" && !attr.IsRequired {
			// Add empty value so the variable exists in the script
			parsedValues[attr.MachineName] = ""
			continue
		}

		// Validate required fields
		if value == "" && attr.IsRequired {
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Campo obrigatório: "+attr.Label, http.StatusSeeOther)
			return
		}

		// Parse value based on type
		switch attr.PrimitiveKind {
		case "BOOL":
			parsedValues[attr.MachineName] = value == "1" || value == "true"
		case "INT":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Valor inválido para "+attr.Label, http.StatusSeeOther)
				return
			}
			parsedValues[attr.MachineName] = intVal
		case "REAL":
			realVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Valor inválido para "+attr.Label, http.StatusSeeOther)
				return
			}
			parsedValues[attr.MachineName] = realVal
		case "TEXT":
			// Validate max_length
			if attr.MaxLength != nil && len(value) > *attr.MaxLength {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Campo "+attr.Label+" excede o limite de "+fmt.Sprint(*attr.MaxLength)+" caracteres", http.StatusSeeOther)
				return
			}
			parsedValues[attr.MachineName] = value
		case "DATETIME":
			if value != "" {
				parsedValues[attr.MachineName] = value
			}
		}
	}

	// =========================================================
	// PHASE 2+3: Transaction-wrapped pre_save script + save
	// pre_save script and EAV save share the same transaction
	// =========================================================
	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Erro ao iniciar transação", http.StatusSeeOther)
		return
	}
	// Ensure rollback on any error
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// Create filodb context with transaction for pre_save script
	dbAdapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
	dbCtxWithTx := filodb.NewContext(dbAdapter, &txAdapter{tx: tx})

	// Execute pre_save script with transaction context
	if entityType.PreSave != "" {
		scriptSetup := func(eng *filo.Engine) {
			filo.RegisterStringBuiltins(eng)
			filodb.RegisterDBBuiltins(eng, dbCtxWithTx)
		}
		modifiedValues, userError, execErr := db.ExecutePreSaveScriptWithSetup(entityType, parsedValues, scriptSetup)
		if execErr != nil {
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Erro no script: "+execErr.Error(), http.StatusSeeOther)
			return
		}
		if userError != "" {
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message="+userError, http.StatusSeeOther)
			return
		}
		parsedValues = modifiedValues
	}

	// Create record using transaction
	refID := utils.NewOpaqueID()
	var recordID int64
	var recordRefID string
	err = tx.QueryRow(`INSERT INTO eav_records (reference_id, entity_type_id, status, rev) VALUES (?, ?, 'draft', 1) RETURNING id, reference_id`, refID, entityType.ID).Scan(&recordID, &recordRefID)
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message=Erro ao criar registro", http.StatusSeeOther)
		return
	}

	// Save each value using transaction
	for machineName, rawValue := range parsedValues {
		attr, ok := attrByMachine[machineName]
		if !ok {
			continue // Skip values for unknown attributes
		}

		var vBool *bool
		var vInt *int64
		var vReal *float64
		var vText *string
		var vDatetime *string

		// Convert back to typed pointers
		switch attr.PrimitiveKind {
		case "BOOL":
			if v, ok := rawValue.(bool); ok {
				vBool = &v
			}
		case "INT":
			if v, ok := rawValue.(int64); ok {
				vInt = &v
			}
		case "REAL":
			if v, ok := rawValue.(float64); ok {
				vReal = &v
			} else if v, ok := rawValue.(int64); ok {
				fv := float64(v)
				vReal = &fv
			}
		case "TEXT":
			if v, ok := rawValue.(string); ok {
				vText = &v
			}
		case "DATETIME":
			if v, ok := rawValue.(string); ok {
				vDatetime = &v
			}
		}

		// Validate unique constraint (using transaction)
		var uniqueValue interface{}
		switch attr.PrimitiveKind {
		case "BOOL":
			uniqueValue = vBool
		case "INT":
			uniqueValue = vInt
		case "REAL":
			uniqueValue = vReal
		case "TEXT":
			uniqueValue = vText
		case "DATETIME":
			uniqueValue = vDatetime
		}

		if attr.IsUnique && uniqueValue != nil {
			isUnique, err := db.Storage.CheckEAVValueUnique(attr.ID, attr.PrimitiveKind, uniqueValue, 0)
			if err != nil {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Erro ao validar unicidade: "+err.Error(), http.StatusSeeOther)
				return
			}
			if !isUnique {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=O valor já existe para o campo "+attr.Label, http.StatusSeeOther)
				return
			}
		}

		// Upsert value using transaction
		err = tx.Exec(`
			INSERT INTO eav_values (record_id, attribute_id, v_bool, v_int, v_real, v_text, v_datetime)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(record_id, attribute_id) DO UPDATE SET
				v_bool = excluded.v_bool,
				v_int = excluded.v_int,
				v_real = excluded.v_real,
				v_text = excluded.v_text,
				v_datetime = excluded.v_datetime,
				updated_at = datetime('now')
		`, recordID, attr.ID, vBool, vInt, vReal, vText, vDatetime)
		if err != nil {
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Erro ao salvar valor: "+err.Error(), http.StatusSeeOther)
			return
		}
	}

	// Set status to 'active' using transaction (must increment rev per trigger constraint)
	err = tx.Exec(`UPDATE eav_records SET status = 'active', rev = rev + 1 WHERE id = ?`, recordID)
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Erro ao ativar registro: "+err.Error(), http.StatusSeeOther)
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Erro ao salvar: "+err.Error(), http.StatusSeeOther)
		return
	}
	committed = true

	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message=Registro criado com sucesso", http.StatusSeeOther)
}

// ToolsDatabaseSchemaEAVRecordEdit shows edit form
func (h *Handlers) ToolsDatabaseSchemaEAVRecordEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	entityRefID := r.PathValue("id")
	recordRefID := r.PathValue("record_id")

	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		http.Error(w, "Entity type not found", http.StatusNotFound)
		return
	}

	record, err := db.Storage.GetEAVRecordByRefID(recordRefID)
	if err != nil {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to fetch attributes", http.StatusInternalServerError)
		return
	}

	values, err := db.Storage.GetEAVValuesByRecordID(record.ID)
	if err != nil {
		http.Error(w, "failed to fetch values", http.StatusInternalServerError)
		return
	}

	// Convert to map
	valueMap := make(map[string]interface{})
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
				valueMap[attr.MachineName] = *attr.DefaultVDatetime
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
			for k, v := range modifiedValues {
				valueMap[k] = v
			}
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
		Config       config.Config
		CurrentPage  string
		EntityType   *db.EAVEntityType
		Attributes   []db.EAVAttribute
		Record       *db.EAVRecord
		Values       map[string]interface{}
		Message      string
		PosLoadError string
	}{
		Authed:       true,
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

	err = h.templates(w, "tools_database_schema_eav_record_edit.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// ToolsDatabaseSchemaEAVRecordUpdate updates existing record
func (h *Handlers) ToolsDatabaseSchemaEAVRecordUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	entityRefID := r.PathValue("id")
	recordRefID := r.PathValue("record_id")

	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		http.Error(w, "Entity type not found", http.StatusNotFound)
		return
	}

	record, err := db.Storage.GetEAVRecordByRefID(recordRefID)
	if err != nil {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to fetch attributes", http.StatusInternalServerError)
		return
	}

	// Build attribute lookup by machine_name for later use
	attrByMachine := make(map[string]db.EAVAttribute)
	for _, attr := range attributes {
		attrByMachine[attr.MachineName] = attr
	}

	// Get current rev from form
	currentRev, err := strconv.Atoi(r.FormValue("rev"))
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Erro: rev inválida", http.StatusSeeOther)
		return
	}

	// =========================================================
	// PHASE 1: Parse all values into a map (without saving)
	// =========================================================
	parsedValues := make(db.EAVRecordValues)

	for _, attr := range attributes {
		value := r.FormValue("attr_" + attr.MachineName)

		// Validate required
		if value == "" && attr.IsRequired {
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Campo obrigatório: "+attr.Label, http.StatusSeeOther)
			return
		}

		// For empty non-required fields, still set the variable so scripts can check it
		if value == "" {
			parsedValues[attr.MachineName] = ""
			continue
		}

		// Parse value based on type
		switch attr.PrimitiveKind {
		case "BOOL":
			parsedValues[attr.MachineName] = value == "1" || value == "true"
		case "INT":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Valor inválido para "+attr.Label, http.StatusSeeOther)
				return
			}
			parsedValues[attr.MachineName] = intVal
		case "REAL":
			realVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Valor inválido para "+attr.Label, http.StatusSeeOther)
				return
			}
			parsedValues[attr.MachineName] = realVal
		case "TEXT":
			// Validate max_length
			if attr.MaxLength != nil && len(value) > *attr.MaxLength {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Campo "+attr.Label+" excede o limite de "+fmt.Sprint(*attr.MaxLength)+" caracteres", http.StatusSeeOther)
				return
			}
			parsedValues[attr.MachineName] = value
		case "DATETIME":
			parsedValues[attr.MachineName] = value
		}
	}

	// =========================================================
	// PHASE 2: Execute pre_save script with transaction (if defined)
	// =========================================================
	if entityType.PreSave != "" {
		// Start transaction for pre_save script
		tx, err := db.Storage.BeginTransaction()
		if err != nil {
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Erro ao iniciar transação", http.StatusSeeOther)
			return
		}
		committed := false
		defer func() {
			if !committed {
				tx.Rollback()
			}
		}()

		// Create filodb context with transaction for pre_save script
		dbAdapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
		dbCtxWithTx := filodb.NewContext(dbAdapter, &txAdapter{tx: tx})

		scriptSetup := func(eng *filo.Engine) {
			filo.RegisterStringBuiltins(eng)
			filodb.RegisterDBBuiltins(eng, dbCtxWithTx)
		}
		modifiedValues, userError, execErr := db.ExecutePreSaveScriptWithSetup(entityType, parsedValues, scriptSetup)
		if execErr != nil {
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Erro no script: "+execErr.Error(), http.StatusSeeOther)
			return
		}
		if userError != "" {
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message="+userError, http.StatusSeeOther)
			return
		}
		// Commit pre_save transaction
		if err := tx.Commit(); err != nil {
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Erro ao salvar script: "+err.Error(), http.StatusSeeOther)
			return
		}
		committed = true
		// Apply modified values
		parsedValues = modifiedValues
	}

	// =========================================================
	// PHASE 3: Save values with optimistic locking
	// =========================================================
	for machineName, rawValue := range parsedValues {
		attr, ok := attrByMachine[machineName]
		if !ok {
			continue // Skip values for unknown attributes
		}

		var vBool *bool
		var vInt *int64
		var vReal *float64
		var vText *string
		var vDatetime *string

		// Convert back to typed pointers
		switch attr.PrimitiveKind {
		case "BOOL":
			if v, ok := rawValue.(bool); ok {
				vBool = &v
			}
		case "INT":
			if v, ok := rawValue.(int64); ok {
				vInt = &v
			}
		case "REAL":
			if v, ok := rawValue.(float64); ok {
				vReal = &v
			} else if v, ok := rawValue.(int64); ok {
				// Handle case where script returned int instead of float
				fv := float64(v)
				vReal = &fv
			}
		case "TEXT":
			if v, ok := rawValue.(string); ok {
				vText = &v
			}
		case "DATETIME":
			if v, ok := rawValue.(string); ok {
				vDatetime = &v
			}
		}

		// Validate unique constraint (exclude current record)
		var uniqueValue interface{}
		switch attr.PrimitiveKind {
		case "BOOL":
			uniqueValue = vBool
		case "INT":
			uniqueValue = vInt
		case "REAL":
			uniqueValue = vReal
		case "TEXT":
			uniqueValue = vText
		case "DATETIME":
			uniqueValue = vDatetime
		}

		if attr.IsUnique && uniqueValue != nil {
			isUnique, err := db.Storage.CheckEAVValueUnique(attr.ID, attr.PrimitiveKind, uniqueValue, record.ID)
			if err != nil {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Erro ao validar unicidade: "+err.Error(), http.StatusSeeOther)
				return
			}
			if !isUnique {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=O valor já existe para o campo "+attr.Label, http.StatusSeeOther)
				return
			}
		}

		// Use UpsertEAVValueWithRev for atomic update
		_, err = db.Storage.UpsertEAVValueWithRev(record.ID, attr.ID, currentRev, vBool, vInt, vReal, vText, vDatetime)
		if err != nil {
			if err == db.ErrConflict {
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Conflito: registro foi modificado por outro usuário. Recarregue a página.", http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Erro ao atualizar: "+err.Error(), http.StatusSeeOther)
			return
		}

		// Update currentRev after first successful update
		currentRev++
	}

	// Always set status to 'active' after successful save
	err = db.Storage.UpdateEAVRecordStatus(record.ID, currentRev, "active")
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Erro ao ativar registro: "+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message=Registro atualizado com sucesso", http.StatusSeeOther)
}

// ToolsDatabaseSchemaEAVRecordDelete soft deletes a record
func (h *Handlers) ToolsDatabaseSchemaEAVRecordDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	entityRefID := r.PathValue("id")
	recordRefID := r.PathValue("record_id")

	record, err := db.Storage.GetEAVRecordByRefID(recordRefID)
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message=Registro não encontrado", http.StatusSeeOther)
		return
	}

	err = db.Storage.SoftDeleteEAVRecord(record.ID)
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message=Erro ao excluir: "+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message=Registro excluído com sucesso", http.StatusSeeOther)
}
