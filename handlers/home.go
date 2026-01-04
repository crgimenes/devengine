package handlers

import (
	"io"
	"mime/multipart"
	"net/http"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
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
	_, _, _, err := auth.Prelude(w, r,
		[]string{
			http.MethodGet,
			http.MethodHead,
		},
		false, // check auth
		false, // check ratelimit
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
		http.Error(w, "message too long", http.StatusBadRequest)
		return
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

	err = h.templates(w, templateName, data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) Tools(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true,  // check auth - must be logged in
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

	// Sysop-only check
	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		http.Error(w, "message too long", http.StatusBadRequest)
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

	err = h.templates(w, "tools.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) ToolsDatabaseSchema(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true,  // check auth - must be logged in
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

	// Sysop-only check
	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		http.Error(w, "message too long", http.StatusBadRequest)
		return
	}

	// Fetch relational tables
	relTables, err := db.Storage.ListRelationalTables()
	if err != nil {
		http.Error(w, "failed to list relational tables", http.StatusInternalServerError)
		return
	}

	// Fetch EAV entity types
	eavTypes, err := db.Storage.ListEAVEntityTypes()
	if err != nil {
		http.Error(w, "failed to list EAV entity types", http.StatusInternalServerError)
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

	// Sort alphabetically by Name
	type byName []SchemaTable
	sort := func(a, b SchemaTable) bool { return a.Name < b.Name }
	sortByName := func(tables []SchemaTable) {
		for i := 0; i < len(tables)-1; i++ {
			for j := i + 1; j < len(tables); j++ {
				if !sort(tables[i], tables[j]) {
					tables[i], tables[j] = tables[j], tables[i]
				}
			}
		}
	}
	sortByName(tables)

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

	err = h.templates(w, "tools_database_schema.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) ToolsForms(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true,  // check auth - must be logged in
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

	// Sysop-only check
	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		http.Error(w, "message too long", http.StatusBadRequest)
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
		CurrentPage: "forms",
	}

	err = h.templates(w, "tools_forms.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) ToolsUsers(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true,  // check auth - must be logged in
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

	// Sysop-only check
	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		http.Error(w, "message too long", http.StatusBadRequest)
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
		CurrentPage: "users",
	}

	err = h.templates(w, "tools_users.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) ToolsMenuEditor(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true,  // check auth - must be logged in
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

	// Sysop-only check
	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		http.Error(w, "message too long", http.StatusBadRequest)
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
		CurrentPage: "menu-editor",
	}

	err = h.templates(w, "tools_menu_editor.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) ToolsDatabaseSchemaEAVNew(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet, http.MethodPost},
		true,  // check auth - must be logged in
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

	// Sysop-only check
	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
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
			errorMsg = "Nome é obrigatório"
		} else if machineName == "" {
			errorMsg = "Nome da máquina é obrigatório"
		} else if len(name) > 100 {
			errorMsg = "Nome deve ter no máximo 100 caracteres"
		} else if len(machineName) > 100 {
			errorMsg = "Nome da máquina deve ter no múximo 100 caracteres"
		} else if len(description) > 500 {
			errorMsg = "Descrição deve ter no máximo 500 caracteres"
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
					if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_') {
						validMachineName = false
						break
					}
				}
			}
			if !validMachineName {
				errorMsg = "Nome da máquina deve conter apenas letras minúsculas, números e underscores, e começar com letra"
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
			h.templates(w, "tools_database_schema_eav_new.go.tmpl", data)
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
				Error:       "Erro ao criar tabela EAV: " + err.Error(),
				FormData: FormData{
					Name:        name,
					MachineName: machineName,
					Description: description,
					PreSave:     preSave,
					PosLoad:     posLoad,
				},
			}
			h.templates(w, "tools_database_schema_eav_new.go.tmpl", data)
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

	err = h.templates(w, "tools_database_schema_eav_new.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) ToolsDatabaseSchemaEAVEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet, http.MethodPost},
		true,  // check auth - must be logged in
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

	// Sysop-only check
	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get the reference ID from URL path
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing entity type ID", http.StatusBadRequest)
		return
	}

	// Fetch entity type
	entityType, err := db.Storage.GetEAVEntityTypeByRefID(id)
	if err != nil {
		if err == db.ErrNotFound {
			http.Error(w, "Entity type not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to fetch entity type", http.StatusInternalServerError)
		return
	}

	// Fetch attributes for this entity type
	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to fetch attributes", http.StatusInternalServerError)
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
			errorMsg = "Nome é obrigatório"
		} else if len(name) > 100 {
			errorMsg = "Nome deve ter no máximo 100 caracteres"
		} else if len(description) > 500 {
			errorMsg = "Descrição deve ter no máximo 500 caracteres"
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
			h.templates(w, "tools_database_schema_eav_edit.go.tmpl", data)
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
				Error:       "Erro ao atualizar tabela: " + err.Error(),
				Config:      *h.cfg,
				CurrentPage: "database-schema",
				EntityID:    id,
				EntityType:  entityType,
				Attributes:  attributes,
			}
			h.templates(w, "tools_database_schema_eav_edit.go.tmpl", data)
			return
		}

		// Redirect with success message
		http.Redirect(w, r, "/tools/database-schema/eav/"+updatedET.ReferenceID+"/edit?message=Tabela atualizada com sucesso", http.StatusSeeOther)
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

	err = h.templates(w, "tools_database_schema_eav_edit.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
