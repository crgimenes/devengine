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
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/attributes/new", h.ToolsDatabaseSchemaEAVAttributeCreate)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/attributes/{attr_id}/update", h.ToolsDatabaseSchemaEAVAttributeUpdate)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/attributes/{attr_id}/delete", h.ToolsDatabaseSchemaEAVAttributeDelete)
	mux.HandleFunc("/tools/forms", h.ToolsForms)
	mux.HandleFunc("/tools/users", h.ToolsUsers)
	mux.HandleFunc("/tools/menu-editor", h.ToolsMenuEditor)
}
