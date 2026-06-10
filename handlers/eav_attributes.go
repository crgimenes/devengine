package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/log"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
)

// ToolsDatabaseSchemaEAVAttributeNew shows the form to create a new attribute
func (h *Handlers) ToolsDatabaseSchemaEAVAttributeNew(w http.ResponseWriter, r *http.Request) {
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

	data := struct {
		Authed      bool
		User        *db.User
		Config      *config.Config
		CurrentPage string
		EntityType  *db.EAVEntityType
		Attribute   *db.EAVAttribute
		Message     string
	}{
		Authed:      authed,
		User:        user,
		Config:      config.Cfg,
		CurrentPage: "tools",
		EntityType:  entityType,
		Attribute:   nil,
		Message:     r.URL.Query().Get("message"),
	}

	err = h.templates(w, "tools_database_schema_eav_attribute_form.go.tmpl", data)
	if err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// ToolsDatabaseSchemaEAVAttributeEdit shows the form to edit an existing attribute
func (h *Handlers) ToolsDatabaseSchemaEAVAttributeEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	entityRefID := r.PathValue("id")
	attrIDStr := r.PathValue("attr_id")

	attrID, err := strconv.ParseInt(attrIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		http.Error(w, "Entity type not found", http.StatusNotFound)
		return
	}

	attribute, err := db.Storage.GetEAVAttributeByID(attrID)
	if err != nil {
		http.Error(w, "Attribute not found", http.StatusNotFound)
		return
	}

	data := struct {
		Authed      bool
		User        *db.User
		Config      *config.Config
		CurrentPage string
		EntityType  *db.EAVEntityType
		Attribute   *db.EAVAttribute
		Message     string
	}{
		Authed:      authed,
		User:        user,
		Config:      config.Cfg,
		CurrentPage: "tools",
		EntityType:  entityType,
		Attribute:   attribute,
		Message:     r.URL.Query().Get("message"),
	}

	err = h.templates(w, "tools_database_schema_eav_attribute_form.go.tmpl", data)
	if err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// ToolsDatabaseSchemaEAVAttributeCreate handles POST requests to create a new attribute
func (h *Handlers) ToolsDatabaseSchemaEAVAttributeCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
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

	// Parse form
	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	helpText := r.FormValue("help_text")
	primitiveKind := r.FormValue("primitive_kind")
	defaultValue := r.FormValue("default_value")
	isRequired := r.FormValue("is_required") == "1"
	isUnique := r.FormValue("is_unique") == "1"
	isIndexed := r.FormValue("is_indexed") == "1"

	// Validation
	if machineName == "" || label == "" || primitiveKind == "" {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+
			"Nome da máquina, rótulo e tipo são obrigatórios", http.StatusSeeOther)
		return
	}

	// Parse default value based on primitive kind
	var defaultVBool *bool
	var defaultVInt *int64
	var defaultVReal *float64
	var defaultVText *string
	var defaultVDatetime *string

	if defaultValue != "" {
		// Normalize decimal separator
		defaultValue = strings.ReplaceAll(defaultValue, ",", ".")

		switch primitiveKind {
		case "BOOL":
			val, err := strconv.ParseInt(defaultValue, 10, 64)
			if err == nil && (val == 0 || val == 1) {
				boolVal := val == 1
				defaultVBool = &boolVal
			}
		case "INT":
			val, err := strconv.ParseInt(defaultValue, 10, 64)
			if err == nil {
				defaultVInt = &val
			}
		case "REAL":
			val, err := strconv.ParseFloat(defaultValue, 64)
			if err == nil {
				defaultVReal = &val
			}
		case "TEXT":
			defaultVText = &defaultValue
		case "DATETIME":
			if defaultValue != "" {
				defaultVDatetime = &defaultValue
			}
		}
	}

	// Get max_length from form (TEXT only)
	var maxLength *int
	if primitiveKind == "TEXT" {
		if maxLenStr := r.FormValue("max_length"); maxLenStr != "" {
			maxLenInt, err := strconv.Atoi(maxLenStr)
			if err == nil && maxLenInt > 0 && maxLenInt <= 65535 {
				maxLength = &maxLenInt
			}
		}
		// Default to 256 if not specified
		if maxLength == nil {
			defaultLen := 256
			maxLength = &defaultLen
		}
	}

	// Create attribute
	_, err = db.Storage.CreateEAVAttribute(
		entityType.ID,
		machineName,
		label,
		helpText,
		primitiveKind,
		isRequired,
		isUnique,
		isIndexed,
		maxLength,
		false, // isComputed
		"",    // computedExpr
		defaultVBool,
		defaultVInt,
		defaultVReal,
		defaultVText,
		defaultVDatetime,
	)
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+
			"Erro ao criar atributo: "+err.Error(), http.StatusSeeOther)
		return
	}

	// Success - redirect back to edit page
	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message=Atributo criado com sucesso", http.StatusSeeOther)
}

// ToolsDatabaseSchemaEAVAttributeDelete handles POST requests to soft-delete an attribute
func (h *Handlers) ToolsDatabaseSchemaEAVAttributeDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
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

	// Get entity type ID and attribute ID from path
	entityRefID := r.PathValue("id")
	attrRefID := r.PathValue("attr_id")
	if entityRefID == "" || attrRefID == "" {
		http.Error(w, "missing IDs", http.StatusBadRequest)
		return
	}

	// Fetch attribute by reference_id to get its ID
	attr, err := db.Storage.GetEAVAttributeByRefID(attrRefID)
	if err != nil {
		if err == db.ErrNotFound {
			http.Error(w, "Attribute not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch attribute", http.StatusInternalServerError)
		return
	}

	// Soft delete
	err = db.Storage.SoftDeleteEAVAttribute(attr.ID)
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+
			"Erro ao excluir atributo: "+err.Error(), http.StatusSeeOther)
		return
	}

	// Success
	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message=Atributo excluído com sucesso", http.StatusSeeOther)
}

// ToolsDatabaseSchemaEAVAttributeUpdate handles POST requests to update an existing attribute
func (h *Handlers) ToolsDatabaseSchemaEAVAttributeUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
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

	// Get entity type ID and attribute ID from path
	entityRefID := r.PathValue("id")
	attrRefID := r.PathValue("attr_id")
	if entityRefID == "" || attrRefID == "" {
		http.Error(w, "missing IDs", http.StatusBadRequest)
		return
	}

	// Fetch attribute to get its internal ID
	attr, err := db.Storage.GetEAVAttributeByRefID(attrRefID)
	if err != nil {
		if err == db.ErrNotFound {
			http.Error(w, "Attribute not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch attribute", http.StatusInternalServerError)
		return
	}

	// Parse form
	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	helpText := r.FormValue("help_text")
	primitiveKind := r.FormValue("primitive_kind")
	defaultValue := r.FormValue("default_value")

	isRequired := r.FormValue("is_required") == "1"
	isUnique := r.FormValue("is_unique") == "1"
	isIndexed := r.FormValue("is_indexed") == "1"

	// Validation
	if machineName == "" || label == "" || primitiveKind == "" {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+
			"Nome da máquina, rótulo e tipo são obrigatórios", http.StatusSeeOther)
		return
	}

	// Parse default value based on primitive kind
	var defaultVBool *bool
	var defaultVInt *int64
	var defaultVReal *float64
	var defaultVText *string
	var defaultVDatetime *string

	if defaultValue != "" {
		// Normalize decimal separator
		defaultValue = strings.ReplaceAll(defaultValue, ",", ".")

		switch primitiveKind {
		case "BOOL":
			val, err := strconv.ParseInt(defaultValue, 10, 64)
			if err == nil && (val == 0 || val == 1) {
				boolVal := val == 1
				defaultVBool = &boolVal
			}
		case "INT":
			val, err := strconv.ParseInt(defaultValue, 10, 64)
			if err == nil {
				defaultVInt = &val
			}
		case "REAL":
			val, err := strconv.ParseFloat(defaultValue, 64)
			if err == nil {
				defaultVReal = &val
			}
		case "TEXT":
			defaultVText = &defaultValue
		case "DATETIME":
			if defaultValue != "" {
				defaultVDatetime = &defaultValue
			}
		}
	}

	// Get max_length from form (TEXT only)
	var maxLength *int
	if primitiveKind == "TEXT" {
		if maxLenStr := r.FormValue("max_length"); maxLenStr != "" {
			maxLenInt, err := strconv.Atoi(maxLenStr)
			if err == nil && maxLenInt > 0 && maxLenInt <= 65535 {
				maxLength = &maxLenInt
			}
		}
		// Default to 256 if not specified
		if maxLength == nil {
			defaultLen := 256
			maxLength = &defaultLen
		}
	}

	// Update attribute
	_, err = db.Storage.UpdateEAVAttribute(
		attr.ID,
		machineName,
		label,
		helpText,
		primitiveKind,
		isRequired,
		isUnique,
		isIndexed,
		maxLength,
		false, // isComputed
		"",    // computedExpr
		defaultVBool,
		defaultVInt,
		defaultVReal,
		defaultVText,
		defaultVDatetime,
	)
	if err != nil {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+
			"Erro ao atualizar atributo: "+err.Error(), http.StatusSeeOther)
		return
	}

	// Success
	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message=Atributo atualizado com sucesso", http.StatusSeeOther)
}
