package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
)

// RecordWithValues combines a record with its attribute values
type RecordWithValues struct {
	Record db.EAVRecord
	Values map[string]interface{} // attribute machine_name -> value
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

	data := struct {
		Authed      bool
		User        db.User
		Config      config.Config
		CurrentPage string
		EntityType  *db.EAVEntityType
		Attributes  []db.EAVAttribute
		Record      *db.EAVRecord
		Values      map[string]interface{}
		Message     string
	}{
		Authed:      true,
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "database-schema",
		EntityType:  entityType,
		Attributes:  attributes,
		Record:      nil, // New record
		Values:      values,
		Message:     r.URL.Query().Get("message"),
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

	// Create record with status='draft'
	record, err := db.Storage.CreateEAVRecord(entityType.ID)
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records?message=Erro ao criar registro", http.StatusSeeOther)
		return
	}

	// Parse and save values
	for _, attr := range attributes {
		value := r.FormValue("attr_" + attr.MachineName)

		// Skip empty non-required fields
		if value == "" && !attr.IsRequired {
			continue
		}

		// Validate required fields
		if value == "" && attr.IsRequired {
			db.Storage.SoftDeleteEAVRecord(record.ID) // Cleanup
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Campo obrigatório: "+attr.Label, http.StatusSeeOther)
			return
		}

		// Parse and upsert value based on type
		var vBool *bool
		var vInt *int64
		var vReal *float64
		var vText *string
		var vDatetime *string

		switch attr.PrimitiveKind {
		case "BOOL":
			boolVal := value == "1" || value == "true"
			vBool = &boolVal
		case "INT":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				db.Storage.SoftDeleteEAVRecord(record.ID)
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Valor inválido para "+attr.Label, http.StatusSeeOther)
				return
			}
			vInt = &intVal
		case "REAL":
			realVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				db.Storage.SoftDeleteEAVRecord(record.ID)
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Valor inválido para "+attr.Label, http.StatusSeeOther)
				return
			}
			vReal = &realVal
		case "TEXT":
			// Validate max_length
			if attr.MaxLength != nil && len(value) > *attr.MaxLength {
				db.Storage.SoftDeleteEAVRecord(record.ID)
				http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Campo "+attr.Label+" excede o limite de "+fmt.Sprint(*attr.MaxLength)+" caracteres", http.StatusSeeOther)
				return
			}
			vText = &value
		case "DATETIME":
			if value != "" {
				vDatetime = &value
			}
		}

		// Validate unique constraint
		if attr.IsUnique && value != "" {
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

			if uniqueValue != nil {
				isUnique, err := db.Storage.CheckEAVValueUnique(attr.ID, attr.PrimitiveKind, uniqueValue, 0)
				if err != nil {
					db.Storage.SoftDeleteEAVRecord(record.ID)
					http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Erro ao validar unicidade: "+err.Error(), http.StatusSeeOther)
					return
				}
				if !isUnique {
					db.Storage.SoftDeleteEAVRecord(record.ID)
					http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=O valor já existe para o campo "+attr.Label, http.StatusSeeOther)
					return
				}
			}
		}

		err = db.Storage.UpsertEAVValue(record.ID, attr.ID, vBool, vInt, vReal, vText, vDatetime)
		if err != nil {
			db.Storage.SoftDeleteEAVRecord(record.ID)
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Erro ao salvar valor: "+err.Error(), http.StatusSeeOther)
			return
		}
	}

	// Set status to 'active' after all values saved successfully
	err = db.Storage.UpdateEAVRecordStatus(record.ID, record.Rev, "active")
	if err != nil {
		db.Storage.SoftDeleteEAVRecord(record.ID)
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/new?message=Erro ao ativar registro: "+err.Error(), http.StatusSeeOther)
		return
	}

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

	// Get message from query
	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed      bool
		User        db.User
		Config      config.Config
		CurrentPage string
		EntityType  *db.EAVEntityType
		Attributes  []db.EAVAttribute
		Record      *db.EAVRecord
		Values      map[string]interface{}
		Message     string
	}{
		Authed:      true,
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "database-schema",
		EntityType:  entityType,
		Attributes:  attributes,
		Record:      record,
		Values:      valueMap,
		Message:     message,
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

	// Get current rev from form
	currentRev, err := strconv.Atoi(r.FormValue("rev"))
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Erro: rev inválida", http.StatusSeeOther)
		return
	}

	// Update values with optimistic locking
	for _, attr := range attributes {
		value := r.FormValue("attr_" + attr.MachineName)

		// Validate required
		if value == "" && attr.IsRequired {
			http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Campo obrigatório: "+attr.Label, http.StatusSeeOther)
			return
		}

		// Parse value
		var vBool *bool
		var vInt *int64
		var vReal *float64
		var vText *string
		var vDatetime *string

		if value != "" {
			switch attr.PrimitiveKind {
			case "BOOL":
				boolVal := value == "1" || value == "true"
				vBool = &boolVal
			case "INT":
				intVal, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Valor inválido para "+attr.Label, http.StatusSeeOther)
					return
				}
				vInt = &intVal
			case "REAL":
				realVal, err := strconv.ParseFloat(value, 64)
				if err != nil {
					http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Valor inválido para "+attr.Label, http.StatusSeeOther)
					return
				}
				vReal = &realVal
			case "TEXT":
				// Validate max_length
				if attr.MaxLength != nil && len(value) > *attr.MaxLength {
					http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/records/"+recordRefID+"/edit?message=Campo "+attr.Label+" excede o limite de "+fmt.Sprint(*attr.MaxLength)+" caracteres", http.StatusSeeOther)
					return
				}
				vText = &value
			case "DATETIME":
				vDatetime = &value
			}
		}

		// Validate unique constraint (exclude current record)
		if attr.IsUnique && value != "" {
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

			if uniqueValue != nil {
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
	// Note: currentRev was already incremented in the loop above
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
