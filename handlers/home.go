package handlers

import (
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/session"
)

type TemplateExecutor func(io.Writer, string, any) error

type FileValidator func(multipart.File, *multipart.FileHeader, []string, []string, int64) (string, int64, error)

type FileUtilities struct {
	Validate     FileValidator
	DataPath     func(*db.User) (string, error)
	SaveMetadata func(*db.File) (*db.File, error)
	NewFilename  func() string
}

type Dependencies struct {
	Config        *config.Config
	Templates     TemplateExecutor
	FileUtilities FileUtilities
}

type Handlers struct {
	cfg       *config.Config
	templates TemplateExecutor
	files     FileUtilities
}

func New(deps Dependencies) *Handlers {
	if deps.Config == nil {
		deps.Config = config.Cfg
	}

	return &Handlers{
		cfg:       deps.Config,
		templates: deps.Templates,
		files:     deps.FileUtilities,
	}
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	// "/" is the mux catch-all: any path no other handler claims lands here
	// and must answer 404, not the home page.
	if r.URL.Path != "/" {
		h.notFound(w, r)
		return
	}

	_, _, _, err := auth.Prelude(w, r,
		[]string{
			http.MethodGet,
			http.MethodHead,
		},
		false, // check auth
		true,  // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "Home", err)
		return
	}

	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", "0")
		return
	}

	sid, ok := session.GetCookie(r)
	u := db.User{}
	authed := false
	if ok {
		got, ok := session.Get(sid)
		if ok {
			u, authed = got, true
		}
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		h.errorPage(w, r, http.StatusBadRequest, "message too long")
		return
	}

	// For authenticated users, check for "init" form and render directly
	if authed {
		initForm, err := db.Storage.GetFormByMachineName("init")
		if err == nil && initForm != nil {
			// Render form directly (no redirect for better UX)
			r.SetPathValue("machineName", "init")
			h.FormsRuntimeNew(w, r)
			return
		}
	}

	data := struct {
		Authed  bool
		User    db.User
		Error   string
		Message string
		Config  config.Config
	}{
		Authed:  authed,
		User:    u,
		Message: message,
		Config:  *h.cfg,
	}

	templateName := "index.go.tmpl"
	if authed {
		templateName = "dashboard.go.tmpl"
	}

	h.render(w, templateName, data)
}

func (h *Handlers) Tools(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, // check auth - must be logged in
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "Tools", err)
		return
	}

	if !authed {
		return
	}

	// Sysop-only check
	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		h.errorPage(w, r, http.StatusBadRequest, "message too long")
		return
	}

	data := struct {
		Authed      bool
		User        db.User
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
	}{
		Authed:      true,
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "tools",
	}

	h.render(w, "tools.go.tmpl", data)
}

func (h *Handlers) ToolsDatabaseSchema(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, // check auth - must be logged in
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "ToolsDatabaseSchema", err)
		return
	}

	if !authed {
		return
	}

	// Sysop-only check
	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		h.errorPage(w, r, http.StatusBadRequest, "message too long")
		return
	}

	// Fetch relational tables
	relTables, err := db.Storage.ListRelationalTables()
	if err != nil {
		h.serverError(w, r, "failed to list relational tables", err)
		return
	}

	// Fetch EAV entity types
	eavTypes, err := db.Storage.ListEAVEntityTypes()
	if err != nil {
		h.serverError(w, r, "failed to list EAV entity types", err)
		return
	}

	// SchemaTable combines relational and EAV tables for display
	type SchemaTable struct {
		Name        string // table name or entity type machine_name
		DisplayName string // table name or entity type name
		IsEAV       bool
		ReferenceID string // only for EAV types
	}

	// Combine into a single list
	var tables []SchemaTable
	for _, t := range relTables {
		tables = append(tables, SchemaTable{
			Name:        t.Name,
			DisplayName: t.Name,
			IsEAV:       false,
		})
	}
	for _, et := range eavTypes {
		tables = append(tables, SchemaTable{
			Name:        et.MachineName,
			DisplayName: et.Name,
			IsEAV:       true,
			ReferenceID: et.ReferenceID,
		})
	}

	slices.SortFunc(tables, func(a, b SchemaTable) int {
		return strings.Compare(a.Name, b.Name)
	})

	data := struct {
		Authed      bool
		User        db.User
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Tables      []SchemaTable
	}{
		Authed:      true,
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "database-schema",
		Tables:      tables,
	}

	h.render(w, "tools_database_schema.go.tmpl", data)
}

func (h *Handlers) ToolsDatabaseSchemaEAVNew(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet, http.MethodPost},
		true, // check auth - must be logged in
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "ToolsDatabaseSchemaEAVNew", err)
		return
	}

	if !authed {
		return
	}

	// Sysop-only check
	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	type FormData struct {
		Name        string
		MachineName string
		Description string
		PreSave     string
		PosLoad     string
	}

	// Handle POST - Create entity type
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		machineName := r.FormValue("machine_name")
		description := r.FormValue("description")
		preSave := r.FormValue("pre_save")
		posLoad := r.FormValue("pos_load")

		// Validation
		var errorMsg string
		if name == "" {
			errorMsg = i18n.T("Name is required")
		} else if machineName == "" {
			errorMsg = i18n.T("Machine name is required")
		} else if len(name) > 100 {
			errorMsg = i18n.T("Name must be at most 100 characters")
		} else if len(machineName) > 100 {
			errorMsg = i18n.T("Machine name must be at most 100 characters")
		} else if len(description) > 500 {
			errorMsg = i18n.T("Description must be at most 500 characters")
		}

		// Validate machine_name format
		if errorMsg == "" {
			validMachineName := true
			if len(machineName) == 0 {
				validMachineName = false
			} else {
				// Must start with letter
				if machineName[0] < 'a' || machineName[0] > 'z' {
					validMachineName = false
				}
				// Only lowercase letters, numbers, underscores
				for _, ch := range machineName {
					if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '_' {
						validMachineName = false
						break
					}
				}
			}
			if !validMachineName {
				errorMsg = i18n.T("Machine name must contain only lowercase letters, numbers and underscores, and start with a letter")
			}
		}

		if errorMsg != "" {
			// Re-render form with error
			data := struct {
				Authed      bool
				User        db.User
				Config      config.Config
				CurrentPage string
				Error       string
				FormData    FormData
			}{
				Authed:      true,
				User:        *user,
				Config:      *h.cfg,
				CurrentPage: "database-schema",
				Error:       errorMsg,
				FormData: FormData{
					Name:        name,
					MachineName: machineName,
					Description: description,
					PreSave:     preSave,
					PosLoad:     posLoad,
				},
			}
			h.render(w, "tools_database_schema_eav_new.go.tmpl", data)
			return
		}

		// Create entity type
		entityType, err := db.Storage.CreateEAVEntityType(name, machineName, description, preSave, posLoad)
		if err != nil {
			data := struct {
				Authed      bool
				User        db.User
				Config      config.Config
				CurrentPage string
				Error       string
				FormData    FormData
			}{
				Authed:      true,
				User:        *user,
				Config:      *h.cfg,
				CurrentPage: "database-schema",
				Error:       i18n.T("Could not create the EAV table (ref %s)", logRef("CreateEAVEntityType", err)),
				FormData: FormData{
					Name:        name,
					MachineName: machineName,
					Description: description,
					PreSave:     preSave,
					PosLoad:     posLoad,
				},
			}
			h.render(w, "tools_database_schema_eav_new.go.tmpl", data)
			return
		}

		// Redirect to edit page
		http.Redirect(w, r, "/tools/database-schema/eav/"+entityType.ReferenceID+"/edit", http.StatusSeeOther)
		return
	}

	// Handle GET - Show form
	data := struct {
		Authed      bool
		User        db.User
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		FormData    FormData
	}{
		Authed:      true,
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "database-schema",
		FormData:    FormData{},
	}

	h.render(w, "tools_database_schema_eav_new.go.tmpl", data)
}

func (h *Handlers) ToolsDatabaseSchemaEAVEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet, http.MethodPost},
		true, // check auth - must be logged in
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "ToolsDatabaseSchemaEAVEdit", err)
		return
	}

	if !authed {
		return
	}

	// Sysop-only check
	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	// Get the reference ID from URL path
	id := r.PathValue("id")
	if id == "" {
		h.errorPage(w, r, http.StatusBadRequest, "missing entity type ID")
		return
	}

	// Fetch entity type
	entityType, err := db.Storage.GetEAVEntityTypeByRefID(id)
	if err != nil {
		if err == db.ErrNotFound {
			h.notFound(w, r)
			return
		}
		h.serverError(w, r, "failed to fetch entity type", err)
		return
	}

	// Fetch attributes for this entity type
	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		h.serverError(w, r, "failed to fetch attributes", err)
		return
	}

	// Handle POST - Update entity type
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		description := r.FormValue("description")
		preSave := r.FormValue("pre_save")
		posLoad := r.FormValue("pos_load")

		// Validation
		var errorMsg string
		if name == "" {
			errorMsg = i18n.T("Name is required")
		} else if len(name) > 100 {
			errorMsg = i18n.T("Name must be at most 100 characters")
		} else if len(description) > 500 {
			errorMsg = i18n.T("Description must be at most 500 characters")
		}

		if errorMsg != "" {
			data := struct {
				Authed      bool
				User        db.User
				Error       string
				Message     string
				Config      config.Config
				CurrentPage string
				EntityID    string
				EntityType  *db.EAVEntityType
				Attributes  []db.EAVAttribute
			}{
				Authed:      true,
				User:        *user,
				Error:       errorMsg,
				Config:      *h.cfg,
				CurrentPage: "database-schema",
				EntityID:    id,
				EntityType:  entityType,
				Attributes:  attributes,
			}
			h.render(w, "tools_database_schema_eav_edit.go.tmpl", data)
			return
		}

		// Update entity type
		updatedET, err := db.Storage.UpdateEAVEntityType(entityType.ID, name, description, preSave, posLoad)
		if err != nil {
			data := struct {
				Authed      bool
				User        db.User
				Error       string
				Message     string
				Config      config.Config
				CurrentPage string
				EntityID    string
				EntityType  *db.EAVEntityType
				Attributes  []db.EAVAttribute
			}{
				Authed:      true,
				User:        *user,
				Error:       i18n.T("Could not update the table (ref %s)", logRef("UpdateEAVEntityType", err)),
				Config:      *h.cfg,
				CurrentPage: "database-schema",
				EntityID:    id,
				EntityType:  entityType,
				Attributes:  attributes,
			}
			h.render(w, "tools_database_schema_eav_edit.go.tmpl", data)
			return
		}

		// Redirect with success message
		http.Redirect(w, r, "/tools/database-schema/eav/"+updatedET.ReferenceID+"/edit?message="+i18n.T("Table updated successfully"), http.StatusSeeOther)
		return
	}

	// Handle GET - Show form
	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed      bool
		User        db.User
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		EntityID    string
		EntityType  *db.EAVEntityType
		Attributes  []db.EAVAttribute
	}{
		Authed:      true,
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "database-schema",
		EntityID:    id,
		EntityType:  entityType,
		Attributes:  attributes,
	}

	h.render(w, "tools_database_schema_eav_edit.go.tmpl", data)
}
