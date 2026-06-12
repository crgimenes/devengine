package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
)

// ToolsDatabaseSchemaEAVAttributeNew shows the form to create a new attribute
func (h *Handlers) ToolsDatabaseSchemaEAVAttributeNew(w http.ResponseWriter, r *http.Request) {
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

	data := struct {
		Authed      bool
		Locale      string
		User        *db.User
		Config      *config.Config
		CurrentPage string
		EntityType  *db.EAVEntityType
		Attribute   *db.EAVAttribute
		Message     string
	}{
		Authed:      authed,
		Locale:      auth.RequestLocale(r),
		User:        user,
		Config:      config.Cfg,
		CurrentPage: "tools",
		EntityType:  entityType,
		Attribute:   nil,
		Message:     r.URL.Query().Get("message"),
	}

	h.render(w, "tools_database_schema_eav_attribute_form.go.tmpl", data)
}

// ToolsDatabaseSchemaEAVAttributeEdit shows the form to edit an existing attribute
func (h *Handlers) ToolsDatabaseSchemaEAVAttributeEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		h.forbidden(w, r)
		return
	}

	entityRefID := r.PathValue("id")
	attrRefID := r.PathValue("attr_id")

	entityType, err := db.Storage.GetEAVEntityTypeByRefID(entityRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	// External URLs carry reference ids, never internal numeric ids.
	attribute, err := db.Storage.GetEAVAttributeByRefID(attrRefID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	data := struct {
		Authed      bool
		Locale      string
		User        *db.User
		Config      *config.Config
		CurrentPage string
		EntityType  *db.EAVEntityType
		Attribute   *db.EAVAttribute
		Message     string
	}{
		Authed:      authed,
		Locale:      auth.RequestLocale(r),
		User:        user,
		Config:      config.Cfg,
		CurrentPage: "tools",
		EntityType:  entityType,
		Attribute:   attribute,
		Message:     r.URL.Query().Get("message"),
	}

	h.render(w, "tools_database_schema_eav_attribute_form.go.tmpl", data)
}

// ToolsDatabaseSchemaEAVAttributeCreate handles POST requests to create a new attribute
func (h *Handlers) ToolsDatabaseSchemaEAVAttributeCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, // check auth
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "ToolsDatabaseSchemaEAVAttributeCreate", err)
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

	// Parse form
	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	helpText := r.FormValue("help_text")
	primitiveKind := r.FormValue("primitive_kind")
	defaultValue := r.FormValue("default_value")
	isRequired := r.FormValue("is_required") == "1"
	isComputed := r.FormValue("is_computed") == "1"
	computedExpr := strings.TrimSpace(r.FormValue("computed_expr"))
	isUnique := r.FormValue("is_unique") == "1"
	isIndexed := r.FormValue("is_indexed") == "1"

	// Validation
	if machineName == "" || label == "" || primitiveKind == "" {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+
			tr(r, "Machine name, label and type are required"), http.StatusSeeOther)
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
		isComputed,
		computedExpr,
		defaultVBool,
		defaultVInt,
		defaultVReal,
		defaultVText,
		defaultVDatetime,
	)
	if err != nil {
		ref := logRef("create attribute", err)
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+
			tr(r, "Could not create the attribute (ref %s)", ref), http.StatusSeeOther)
		return
	}

	// Success - redirect back to edit page
	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+tr(r, "Attribute created successfully"), http.StatusSeeOther)
}

// ToolsDatabaseSchemaEAVAttributeDelete handles POST requests to soft-delete an attribute
func (h *Handlers) ToolsDatabaseSchemaEAVAttributeDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, // check auth
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "ToolsDatabaseSchemaEAVAttributeDelete", err)
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

	// Get entity type ID and attribute ID from path
	entityRefID := r.PathValue("id")
	attrRefID := r.PathValue("attr_id")
	if entityRefID == "" || attrRefID == "" {
		h.errorPage(w, r, http.StatusBadRequest, "missing IDs")
		return
	}

	// Fetch attribute by reference_id to get its ID
	attr, err := db.Storage.GetEAVAttributeByRefID(attrRefID)
	if err != nil {
		if err == db.ErrNotFound {
			h.notFound(w, r)
			return
		}
		h.serverError(w, r, "failed to fetch attribute", err)
		return
	}

	// Soft delete
	err = db.Storage.SoftDeleteEAVAttribute(attr.ID)
	if err != nil {
		ref := logRef("delete attribute", err)
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+
			tr(r, "Could not delete the attribute (ref %s)", ref), http.StatusSeeOther)
		return
	}

	// Success
	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+tr(r, "Attribute deleted successfully"), http.StatusSeeOther)
}

// ToolsDatabaseSchemaEAVAttributeUpdate handles POST requests to update an existing attribute
func (h *Handlers) ToolsDatabaseSchemaEAVAttributeUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, // check auth
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "ToolsDatabaseSchemaEAVAttributeUpdate", err)
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

	// Get entity type ID and attribute ID from path
	entityRefID := r.PathValue("id")
	attrRefID := r.PathValue("attr_id")
	if entityRefID == "" || attrRefID == "" {
		h.errorPage(w, r, http.StatusBadRequest, "missing IDs")
		return
	}

	// Fetch attribute to get its internal ID
	attr, err := db.Storage.GetEAVAttributeByRefID(attrRefID)
	if err != nil {
		if err == db.ErrNotFound {
			h.notFound(w, r)
			return
		}
		h.serverError(w, r, "failed to fetch attribute", err)
		return
	}

	// Parse form
	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	helpText := r.FormValue("help_text")
	primitiveKind := r.FormValue("primitive_kind")
	defaultValue := r.FormValue("default_value")

	isRequired := r.FormValue("is_required") == "1"
	isComputed := r.FormValue("is_computed") == "1"
	computedExpr := strings.TrimSpace(r.FormValue("computed_expr"))
	isUnique := r.FormValue("is_unique") == "1"
	isIndexed := r.FormValue("is_indexed") == "1"

	// Validation
	if machineName == "" || label == "" || primitiveKind == "" {
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+
			tr(r, "Machine name, label and type are required"), http.StatusSeeOther)
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
		isComputed,
		computedExpr,
		defaultVBool,
		defaultVInt,
		defaultVReal,
		defaultVText,
		defaultVDatetime,
	)
	if err != nil {
		ref := logRef("update attribute", err)
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+
			tr(r, "Could not update the attribute (ref %s)", ref), http.StatusSeeOther)
		return
	}

	// Success
	http.Redirect(w, r, "/tools/database-schema/eav/"+entityRefID+"/edit?message="+tr(r, "Attribute updated successfully"), http.StatusSeeOther)
}
