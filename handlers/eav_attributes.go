package handlers

import (
	"net/http"
	"strconv"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/db"
)

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
