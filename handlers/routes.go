package handlers

import "net/http"

// Routes registra as rotas de páginas no mux.
func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.Home)
	mux.HandleFunc("/login", h.LoginPage)
	mux.HandleFunc("POST /login/magic_link", h.LoginMagic)
	mux.HandleFunc("GET /link/{token}", h.MagicLink)
	mux.HandleFunc("/me", h.Profile)
	mux.HandleFunc("/tools", h.Tools)
	mux.HandleFunc("/tools/database-schema", h.ToolsDatabaseSchema)
	mux.HandleFunc("/tools/database-schema/eav/new", h.ToolsDatabaseSchemaEAVNew)
	mux.HandleFunc("/tools/database-schema/eav/{id}/edit", h.ToolsDatabaseSchemaEAVEdit)
	mux.HandleFunc("GET /tools/database-schema/eav/{id}/attributes/new", h.ToolsDatabaseSchemaEAVAttributeNew)
	mux.HandleFunc("GET /tools/database-schema/eav/{id}/attributes/{attr_id}/edit", h.ToolsDatabaseSchemaEAVAttributeEdit)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/attributes/new", h.ToolsDatabaseSchemaEAVAttributeCreate)
	mux.HandleFunc("/tools/database-schema/eav/{id}/attributes/{attr_id}/update", h.ToolsDatabaseSchemaEAVAttributeUpdate)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/attributes/{attr_id}/delete", h.ToolsDatabaseSchemaEAVAttributeDelete)

	// Record management
	mux.HandleFunc("/tools/database-schema/eav/{id}/records", h.ToolsDatabaseSchemaEAVRecords)
	mux.HandleFunc("/tools/database-schema/eav/{id}/records/api", h.ToolsDatabaseSchemaEAVRecordsAPI)
	mux.HandleFunc("/tools/database-schema/eav/{id}/records/new", h.ToolsDatabaseSchemaEAVRecordNew)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/records/new", h.ToolsDatabaseSchemaEAVRecordCreate)
	mux.HandleFunc("/tools/database-schema/eav/{id}/records/{record_id}/edit", h.ToolsDatabaseSchemaEAVRecordEdit)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/records/{record_id}/update", h.ToolsDatabaseSchemaEAVRecordUpdate)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/records/{record_id}/delete", h.ToolsDatabaseSchemaEAVRecordDelete)

	mux.HandleFunc("/tools/forms", h.ToolsForms)
	mux.HandleFunc("GET /tools/forms/new", h.ToolsFormsNew)
	mux.HandleFunc("POST /tools/forms/new", h.ToolsFormsCreate)
	mux.HandleFunc("GET /tools/forms/{id}/edit", h.ToolsFormsEdit)
	mux.HandleFunc("POST /tools/forms/{id}/update", h.ToolsFormsUpdate)
	mux.HandleFunc("POST /tools/forms/{id}/delete", h.ToolsFormsDelete)
	mux.HandleFunc("GET /tools/forms/{id}/records", h.ToolsFormsRecords)
	mux.HandleFunc("POST /tools/forms/{id}/elements/new", h.ToolsFormsElementCreate)
	mux.HandleFunc("GET /tools/forms/{id}/elements/{element_id}/edit", h.ToolsFormsElementEdit)
	mux.HandleFunc("POST /tools/forms/{id}/elements/{element_id}/update", h.ToolsFormsElementUpdate)
	mux.HandleFunc("POST /tools/forms/{id}/elements/{element_id}/delete", h.ToolsFormsElementDelete)
	mux.HandleFunc("POST /tools/forms/{id}/elements/{element_id}/up", h.ToolsFormsElementMoveUp)
	mux.HandleFunc("POST /tools/forms/{id}/elements/{element_id}/down", h.ToolsFormsElementMoveDown)

	mux.HandleFunc("/tools/users", h.ToolsUsers)
	mux.HandleFunc("/tools/menu-editor", h.ToolsMenuEditor)

	// Form Runtime (public access to forms)
	mux.HandleFunc("GET /forms/{formRef}/new", h.FormsRuntimeNew)
	mux.HandleFunc("POST /forms/{formRef}/new", h.FormsRuntimeCreate)
	mux.HandleFunc("GET /forms/{formRef}/r/{recordRef}", h.FormsRuntimeEdit)
	mux.HandleFunc("POST /forms/{formRef}/r/{recordRef}", h.FormsRuntimeUpdate)
	mux.HandleFunc("POST /forms/{formRef}/action/{buttonName}", h.FormsRuntimeButtonAction)
}
