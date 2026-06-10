package handlers

import "net/http"

// Routes registers the engine's built-in pages on the mux. Authentication
// routes (login, signup, invite acceptance) are NOT registered here — they
// are application-side and must be wired separately (e.g. by importing
// devengine/auth/basic and calling basic.Routes).
func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.Home)
	mux.HandleFunc("GET /me", h.Profile)
	mux.HandleFunc("POST /me", h.Profile)
	mux.HandleFunc("GET /tools", h.Tools)
	mux.HandleFunc("GET /tools/database-schema", h.ToolsDatabaseSchema)
	mux.HandleFunc("GET /tools/database-schema/eav/new", h.ToolsDatabaseSchemaEAVNew)
	mux.HandleFunc("POST /tools/database-schema/eav/new", h.ToolsDatabaseSchemaEAVNew)
	mux.HandleFunc("GET /tools/database-schema/eav/{id}/edit", h.ToolsDatabaseSchemaEAVEdit)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/edit", h.ToolsDatabaseSchemaEAVEdit)
	mux.HandleFunc("GET /tools/database-schema/eav/{id}/attributes/new", h.ToolsDatabaseSchemaEAVAttributeNew)
	mux.HandleFunc("GET /tools/database-schema/eav/{id}/attributes/{attr_id}/edit", h.ToolsDatabaseSchemaEAVAttributeEdit)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/attributes/new", h.ToolsDatabaseSchemaEAVAttributeCreate)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/attributes/{attr_id}/update", h.ToolsDatabaseSchemaEAVAttributeUpdate)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/attributes/{attr_id}/delete", h.ToolsDatabaseSchemaEAVAttributeDelete)

	// Record management
	mux.HandleFunc("GET /tools/database-schema/eav/{id}/records", h.ToolsDatabaseSchemaEAVRecords)
	mux.HandleFunc("GET /tools/database-schema/eav/{id}/records/api", h.ToolsDatabaseSchemaEAVRecordsAPI)
	mux.HandleFunc("GET /tools/database-schema/eav/{id}/records/new", h.ToolsDatabaseSchemaEAVRecordNew)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/records/new", h.ToolsDatabaseSchemaEAVRecordCreate)
	mux.HandleFunc("GET /tools/database-schema/eav/{id}/records/{record_id}/edit", h.ToolsDatabaseSchemaEAVRecordEdit)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/records/{record_id}/update", h.ToolsDatabaseSchemaEAVRecordUpdate)
	mux.HandleFunc("POST /tools/database-schema/eav/{id}/records/{record_id}/delete", h.ToolsDatabaseSchemaEAVRecordDelete)

	mux.HandleFunc("GET /tools/forms", h.ToolsForms)
	mux.HandleFunc("GET /tools/forms/new", h.ToolsFormsNew)
	mux.HandleFunc("POST /tools/forms/new", h.ToolsFormsCreate)
	mux.HandleFunc("GET /tools/forms/{id}/edit", h.ToolsFormsEdit)
	mux.HandleFunc("POST /tools/forms/{id}/update", h.ToolsFormsUpdate)
	mux.HandleFunc("POST /tools/forms/{id}/delete", h.ToolsFormsDelete)
	mux.HandleFunc("GET /tools/forms/{id}/records", h.ToolsFormsRecords)
	mux.HandleFunc("GET /tools/forms/{id}/records/rows", h.ToolsFormsRecordsRows)
	mux.HandleFunc("GET /tools/forms/{id}/records/export.csv", h.ToolsFormsRecordsExport)
	mux.HandleFunc("POST /tools/forms/{id}/elements/new", h.ToolsFormsElementCreate)
	mux.HandleFunc("GET /tools/forms/{id}/elements/{element_id}/edit", h.ToolsFormsElementEdit)
	mux.HandleFunc("POST /tools/forms/{id}/elements/{element_id}/update", h.ToolsFormsElementUpdate)
	mux.HandleFunc("POST /tools/forms/{id}/elements/{element_id}/delete", h.ToolsFormsElementDelete)
	mux.HandleFunc("POST /tools/forms/{id}/elements/{element_id}/up", h.ToolsFormsElementMoveUp)
	mux.HandleFunc("POST /tools/forms/{id}/elements/{element_id}/down", h.ToolsFormsElementMoveDown)

	mux.HandleFunc("GET /tools/users", h.ToolsUsers)
	mux.HandleFunc("GET /tools/users/{ref}/edit", h.ToolsUsersEdit)
	mux.HandleFunc("POST /tools/users/{ref}/update", h.ToolsUsersUpdate)
	mux.HandleFunc("POST /tools/users/{ref}/reset-password", h.ToolsUsersResetPassword)
	mux.HandleFunc("GET /tools/search-forms", h.ToolsSearchForms)
	mux.HandleFunc("GET /tools/filo", h.ToolsFilo)

	// Menu Editor
	mux.HandleFunc("GET /tools/menu-editor", h.ToolsMenus)
	mux.HandleFunc("GET /tools/menu-editor/new", h.ToolsMenusNew)
	mux.HandleFunc("POST /tools/menu-editor/new", h.ToolsMenusCreate)
	mux.HandleFunc("GET /tools/menu-editor/{id}/edit", h.ToolsMenusEdit)
	mux.HandleFunc("POST /tools/menu-editor/{id}/update", h.ToolsMenusUpdate)
	mux.HandleFunc("POST /tools/menu-editor/{id}/delete", h.ToolsMenusDelete)
	mux.HandleFunc("GET /tools/menu-editor/{id}/preview", h.ToolsMenusPreview)
	mux.HandleFunc("POST /tools/menu-editor/{id}/items/new", h.ToolsMenusItemCreate)
	mux.HandleFunc("GET /tools/menu-editor/{id}/items/{item_id}/edit", h.ToolsMenusItemEdit)
	mux.HandleFunc("POST /tools/menu-editor/{id}/items/{item_id}/update", h.ToolsMenusItemUpdate)
	mux.HandleFunc("POST /tools/menu-editor/{id}/items/{item_id}/delete", h.ToolsMenusItemDelete)
	mux.HandleFunc("POST /tools/menu-editor/{id}/items/{item_id}/up", h.ToolsMenusItemMoveUp)
	mux.HandleFunc("POST /tools/menu-editor/{id}/items/{item_id}/down", h.ToolsMenusItemMoveDown)

	// Form Runtime (public access via /form/{machineName})
	mux.HandleFunc("GET /form/{machineName}", h.FormsRuntimeNew)
	mux.HandleFunc("POST /form/{machineName}", h.FormsRuntimeCreate)
	mux.HandleFunc("GET /form/{machineName}/r/{recordRef}", h.FormsRuntimeEdit)
	mux.HandleFunc("POST /form/{machineName}/r/{recordRef}", h.FormsRuntimeUpdate)
	mux.HandleFunc("POST /form/{machineName}/action/{buttonName}", h.FormsRuntimeButtonAction)
	mux.HandleFunc("GET /form/{machineName}/actions.js", h.FormsRuntimeActionsJS)

	// Menu Actions (public access via /menu/{menuMachineName})
	mux.HandleFunc("GET /menu/{menuMachineName}/actions.js", h.MenuActionsJS)
	mux.HandleFunc("POST /menu/{menuMachineName}/action/{itemMachineName}", h.MenuItemAction)
}
