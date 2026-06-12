package handlers

import (
	"net/http"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/filodb"
	"github.com/crgimenes/devengine/filolog"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/filo"
	"github.com/crgimenes/filo/filostrings"
)

// ToolsMenus shows the menu list page.
func (h *Handlers) ToolsMenus(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, // check auth
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "ToolsMenus", err)
		return
	}

	if !authed {
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		h.errorPage(w, r, http.StatusBadRequest, "message too long")
		return
	}

	menus, err := db.Storage.ListMenus()
	if err != nil {
		h.serverError(w, r, "failed to fetch menus", err)
		return
	}

	data := struct {
		Authed      bool
		User        db.User
		Locale      string
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Menus       []db.Menu
	}{
		Authed:      true,
		Locale:      auth.RequestLocale(r),
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "menu-editor",
		Menus:       menus,
	}

	h.render(w, "tools_menu_editor.go.tmpl", data)
}

// ToolsMenusNew shows the new menu creation page.
func (h *Handlers) ToolsMenusNew(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusNew", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	csrf := session.GenerateCSRFToken(w, r)

	data := struct {
		Authed      bool
		User        db.User
		Locale      string
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Csrf        string
		FormData    struct{ MachineName, Label, Description string }
	}{
		Authed:      true,
		Locale:      auth.RequestLocale(r),
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "menu-editor",
		Csrf:        csrf,
		FormData:    struct{ MachineName, Label, Description string }{},
	}

	h.render(w, "tools_menu_editor_new.go.tmpl", data)
}

// ToolsMenusCreate handles menu creation.
func (h *Handlers) ToolsMenusCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusCreate", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	if !session.ValidateCSRF(r) {
		h.forbidden(w, r)
		return
	}

	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	description := r.FormValue("description")

	// Validation
	if machineName == "" || label == "" {
		csrf := session.GenerateCSRFToken(w, r)
		data := struct {
			Authed      bool
			User        db.User
			Error       string
			Config      config.Config
			CurrentPage string
			Csrf        string
			FormData    struct{ MachineName, Label, Description string }
		}{
			Authed:      true,
			User:        *user,
			Error:       tr(r, "Name and machine name are required"),
			Config:      *h.cfg,
			CurrentPage: "menu-editor",
			Csrf:        csrf,
			FormData:    struct{ MachineName, Label, Description string }{machineName, label, description},
		}
		h.render(w, "tools_menu_editor_new.go.tmpl", data)
		return
	}

	menu, err := db.Storage.CreateMenu(machineName, label, description)
	if err != nil {
		csrf := session.GenerateCSRFToken(w, r)
		data := struct {
			Authed      bool
			User        db.User
			Error       string
			Config      config.Config
			CurrentPage string
			Csrf        string
			FormData    struct{ MachineName, Label, Description string }
		}{
			Authed:      true,
			User:        *user,
			Error:       tr(r, "Could not create the menu (ref %s)", logRef("CreateMenu", err)),
			Config:      *h.cfg,
			CurrentPage: "menu-editor",
			Csrf:        csrf,
			FormData:    struct{ MachineName, Label, Description string }{machineName, label, description},
		}
		h.render(w, "tools_menu_editor_new.go.tmpl", data)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor/"+menu.ReferenceID+"/edit", http.StatusSeeOther)
}

// ToolsMenusEdit shows the menu edit page with items list.
func (h *Handlers) ToolsMenusEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusEdit", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		h.errorPage(w, r, http.StatusBadRequest, "missing menu ID")
		return
	}

	menu, err := db.Storage.GetMenuByRefID(id)
	if err != nil {
		h.notFound(w, r)
		return
	}

	items, err := db.Storage.ListMenuItems(menu.ID)
	if err != nil {
		h.serverError(w, r, "failed to fetch items", err)
		return
	}

	// Sort hierarchically for display
	sortedItems := db.SortMenuItemsHierarchically(items)

	// Build item views with depth and parent label
	type ItemView struct {
		db.MenuItem
		Depth       int
		ParentLabel string
	}

	// Calculate depth for each item
	depthMap := make(map[int64]int)
	labelMap := make(map[int64]string)
	for _, item := range items {
		labelMap[item.ID] = item.Label
	}

	var calculateDepth func(item db.MenuItem) int
	calculateDepth = func(item db.MenuItem) int {
		if item.ParentID == nil {
			return 0
		}
		if d, ok := depthMap[*item.ParentID]; ok {
			return d + 1
		}
		// Find parent and calculate its depth first
		for _, p := range items {
			if p.ID == *item.ParentID {
				return calculateDepth(p) + 1
			}
		}
		return 1
	}

	for _, item := range items {
		depthMap[item.ID] = calculateDepth(item)
	}

	itemViews := make([]ItemView, len(sortedItems))
	for i, item := range sortedItems {
		parentLabel := ""
		if item.ParentID != nil {
			parentLabel = labelMap[*item.ParentID]
		}
		itemViews[i] = ItemView{
			MenuItem:    item,
			Depth:       depthMap[item.ID],
			ParentLabel: parentLabel,
		}
	}

	message := r.URL.Query().Get("message")
	csrf := session.GenerateCSRFToken(w, r)

	data := struct {
		Authed      bool
		User        db.User
		Locale      string
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Menu        *db.Menu
		Items       []ItemView
		AllItems    []db.MenuItem
		Csrf        string
	}{
		Authed:      true,
		Locale:      auth.RequestLocale(r),
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "menu-editor",
		Menu:        menu,
		Items:       itemViews,
		AllItems:    items,
		Csrf:        csrf,
	}

	h.render(w, "tools_menu_editor_edit.go.tmpl", data)
}

// ToolsMenusUpdate handles menu update.
func (h *Handlers) ToolsMenusUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusUpdate", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	if !session.ValidateCSRF(r) {
		h.forbidden(w, r)
		return
	}

	id := r.PathValue("id")
	menu, err := db.Storage.GetMenuByRefID(id)
	if err != nil {
		h.notFound(w, r)
		return
	}

	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	description := r.FormValue("description")

	err = db.Storage.UpdateMenu(menu.ID, machineName, label, description)
	if err != nil {
		http.Redirect(w, r, "/tools/menu-editor/"+id+"/edit?message="+tr(r, "Could not update"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor/"+id+"/edit?message="+tr(r, "Menu updated successfully"), http.StatusSeeOther)
}

// ToolsMenusDelete handles menu deletion.
func (h *Handlers) ToolsMenusDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusDelete", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	id := r.PathValue("id")
	menu, err := db.Storage.GetMenuByRefID(id)
	if err != nil {
		h.notFound(w, r)
		return
	}

	err = db.Storage.SoftDeleteMenu(menu.ID)
	if err != nil {
		http.Redirect(w, r, "/tools/menu-editor?message="+tr(r, "Could not delete the menu"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor?message="+tr(r, "Menu deleted successfully"), http.StatusSeeOther)
}

// ToolsMenusItemCreate handles creating a new menu item.
func (h *Handlers) ToolsMenusItemCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusItemCreate", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	if !session.ValidateCSRF(r) {
		h.forbidden(w, r)
		return
	}

	id := r.PathValue("id")
	menu, err := db.Storage.GetMenuByRefID(id)
	if err != nil {
		h.notFound(w, r)
		return
	}

	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	icon := r.FormValue("icon")

	if machineName == "" {
		http.Redirect(w, r, "/tools/menu-editor/"+id+"/edit?message="+tr(r, "Machine name is required"), http.StatusSeeOther)
		return
	}

	// Parse parent_id (reference ID)
	var parentID *int64
	parentRefID := r.FormValue("parent_id")
	if parentRefID != "" {
		parent, err := db.Storage.GetMenuItemByRefID(parentRefID)
		if err == nil {
			parentID = &parent.ID
		}
	}

	// Get max z_order + 1 for new item at same level
	items, _ := db.Storage.ListMenuItems(menu.ID)
	maxZOrder := 0
	for _, item := range items {
		sameLevel := (parentID == nil && item.ParentID == nil) ||
			(parentID != nil && item.ParentID != nil && *parentID == *item.ParentID)
		if sameLevel && item.ZOrder >= maxZOrder {
			maxZOrder = item.ZOrder + 1
		}
	}

	item, err := db.Storage.CreateMenuItem(menu.ID, parentID, machineName, label, icon, "link", "", "", "", maxZOrder)
	if err != nil {
		ref := logRef("ToolsMenusItemCreate", err)
		http.Redirect(w, r, "/tools/menu-editor/"+id+"/edit?message="+tr(r, "Could not create the item (ref %s)", ref), http.StatusSeeOther)
		return
	}

	// Drop straight into the item editor: a fresh item is a bare link and
	// almost always needs URL/type/code filled in next.
	http.Redirect(w, r,
		"/tools/menu-editor/"+id+"/items/"+item.ReferenceID+"/edit?message="+tr(r, "Item created successfully"),
		http.StatusSeeOther)
}

// ToolsMenusItemEdit shows the item edit page.
func (h *Handlers) ToolsMenusItemEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusItemEdit", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	menuID := r.PathValue("id")
	itemID := r.PathValue("item_id")

	menu, err := db.Storage.GetMenuByRefID(menuID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	item, err := db.Storage.GetMenuItemByRefID(itemID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	// Get all items for parent selection
	allItems, _ := db.Storage.ListMenuItems(menu.ID)

	csrf := session.GenerateCSRFToken(w, r)
	message := r.URL.Query().Get("message")

	data := struct {
		Authed      bool
		User        db.User
		Locale      string
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Menu        *db.Menu
		Item        *db.MenuItem
		AllItems    []db.MenuItem
		Csrf        string
	}{
		Authed:      true,
		Locale:      auth.RequestLocale(r),
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "menu-editor",
		Menu:        menu,
		Item:        item,
		AllItems:    allItems,
		Csrf:        csrf,
	}

	h.render(w, "tools_menu_editor_item_edit.go.tmpl", data)
}

// ToolsMenusItemUpdate handles item update.
func (h *Handlers) ToolsMenusItemUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusItemUpdate", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	if !session.ValidateCSRF(r) {
		h.forbidden(w, r)
		return
	}

	menuID := r.PathValue("id")
	itemID := r.PathValue("item_id")

	item, err := db.Storage.GetMenuItemByRefID(itemID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	icon := r.FormValue("icon")
	itemType := r.FormValue("item_type")
	url := r.FormValue("url")
	jsCode := r.FormValue("js_code")
	filoCode := r.FormValue("filo_code")

	// Parse parent_id
	var parentID *int64
	parentIDStr := r.FormValue("parent_id")
	if parentIDStr != "" && parentIDStr != "0" {
		// Need to get parent by reference ID
		parent, err := db.Storage.GetMenuItemByRefID(parentIDStr)
		if err == nil {
			parentID = &parent.ID
		}
	}

	err = db.Storage.UpdateMenuItem(item.ID, parentID, machineName, label, icon, itemType, url, jsCode, filoCode, item.ZOrder)
	if err != nil {
		http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/items/"+itemID+"/edit?message="+tr(r, "Could not update"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/edit?message="+tr(r, "Item updated successfully"), http.StatusSeeOther)
}

// ToolsMenusItemDelete handles item deletion.
func (h *Handlers) ToolsMenusItemDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusItemDelete", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	menuID := r.PathValue("id")
	itemID := r.PathValue("item_id")

	item, err := db.Storage.GetMenuItemByRefID(itemID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	err = db.Storage.DeleteMenuItem(item.ID)
	if err != nil {
		http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/edit?message="+tr(r, "Could not delete the item"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/edit?message="+tr(r, "Item deleted successfully"), http.StatusSeeOther)
}

// ToolsMenusItemMoveUp moves an item up.
func (h *Handlers) ToolsMenusItemMoveUp(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusItemMoveUp", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	menuID := r.PathValue("id")
	itemID := r.PathValue("item_id")

	item, err := db.Storage.GetMenuItemByRefID(itemID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	_ = db.Storage.MoveMenuItemUp(item.ID)

	http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/edit", http.StatusSeeOther)
}

// ToolsMenusItemMoveDown moves an item down.
func (h *Handlers) ToolsMenusItemMoveDown(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusItemMoveDown", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	menuID := r.PathValue("id")
	itemID := r.PathValue("item_id")

	item, err := db.Storage.GetMenuItemByRefID(itemID)
	if err != nil {
		h.notFound(w, r)
		return
	}

	_ = db.Storage.MoveMenuItemDown(item.ID)

	http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/edit", http.StatusSeeOther)
}

// ToolsMenusPreview shows a preview of the menu.
func (h *Handlers) ToolsMenusPreview(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed {
		if err != nil {
			h.serverError(w, r, "ToolsMenusPreview", err)
		}
		return
	}

	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	id := r.PathValue("id")
	menu, err := db.Storage.GetMenuByRefID(id)
	if err != nil {
		h.notFound(w, r)
		return
	}

	items, err := db.Storage.ListMenuItems(menu.ID)
	if err != nil {
		h.serverError(w, r, "failed to fetch items", err)
		return
	}

	// Build menu tree for the template using the db package function
	menuTree := db.BuildMenuItemTree(items)

	data := struct {
		Authed      bool
		User        db.User
		Locale      string
		Config      config.Config
		CurrentPage string
		Menu        *db.Menu
		Items       []db.MenuItemNode
		AllItems    []db.MenuItem
	}{
		Authed:      true,
		Locale:      auth.RequestLocale(r),
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "menu-editor",
		Menu:        menu,
		Items:       menuTree,
		AllItems:    items,
	}

	h.render(w, "tools_menu_editor_preview.go.tmpl", data)
}

// MenuItemAction handles menu item action execution (Filo code).
// POST /menu/{menuMachineName}/action/{itemMachineName}
func (h *Handlers) MenuItemAction(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil {
		ref := logRef("MenuItemAction prelude", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "erro interno (ref " + ref + ")"})
		return
	}
	if !authed {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Not authenticated"})
		return
	}

	menuMachineName := r.PathValue("menuMachineName")
	itemMachineName := r.PathValue("itemMachineName")

	// Get menu
	menu, err := db.Storage.GetMenuByMachineName(menuMachineName)
	if err != nil || menu == nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Menu not found"})
		return
	}

	// Get menu items
	items, err := db.Storage.ListMenuItems(menu.ID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to load items"})
		return
	}

	// Find the item by machine name
	var menuItem *db.MenuItem
	for i := range items {
		if items[i].MachineName == itemMachineName {
			menuItem = &items[i]
			break
		}
	}
	if menuItem == nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Menu item not found"})
		return
	}

	// Check if there's Filo code to execute
	if menuItem.FiloCode == "" {
		// No Filo code - just return success
		jsonResponse(w, http.StatusOK, map[string]any{
			"error":       "",
			"message":     "",
			"redirect_to": menuItem.URL, // Allow continuing to URL/JS
		})
		return
	}

	// Execute Filo code
	globals := make(map[string]filo.Value)

	// Add menu metadata
	globals["menu:machine_name"] = filo.VString(menu.MachineName)
	globals["menu:label"] = filo.VString(menu.Label)

	// Add item metadata
	globals["item:machine_name"] = filo.VString(menuItem.MachineName)
	globals["item:label"] = filo.VString(menuItem.Label)
	globals["item:url"] = filo.VString(menuItem.URL)

	// Add user metadata
	if user != nil {
		globals["user:id"] = filo.VNum(float64(user.ID))
		globals["user:email"] = filo.VString(user.Email)
		globals["user:sysop"] = filo.VBool(user.Sysop)
	}

	// Control variables
	globals["error"] = filo.VString("")
	globals["message"] = filo.VString("")
	globals["redirect_to"] = filo.VString("")

	// Execute Filo script
	eng := filo.NewEngine()

	// Setup DB context for Filo
	dbAdapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
	dbCtx := filodb.NewContext(dbAdapter, nil)

	// Register builtins
	filostrings.RegisterBuiltins(eng)
	filodb.RegisterDBBuiltins(eng, dbCtx)
	filolog.RegisterLogBuiltins(eng, filolog.NewContext(globals))

	ctx := r.Context()
	cfg := filo.EvalConfig{
		StepLimit:      10000,
		RecursionLimit: 100,
		Timeout:        5 * 1e9, // 5 seconds
	}

	_, newGlobals, runErr := eng.RunScript(ctx, menuItem.FiloCode, globals, cfg)

	// Build response
	response := map[string]any{
		"error":       "",
		"message":     "",
		"redirect_to": "",
	}

	// Check for errors
	if runErr != nil {
		response["error"] = runErr.Error()
	} else {
		// Read control variables from returned globals
		if errVal, ok := newGlobals["error"]; ok {
			if errVal.Kind == filo.KString && errVal.Str != "" {
				response["error"] = errVal.Str
			}
		}
		if msgVal, ok := newGlobals["message"]; ok {
			if msgVal.Kind == filo.KString {
				response["message"] = msgVal.Str
			}
		}
		if redirVal, ok := newGlobals["redirect_to"]; ok {
			if redirVal.Kind == filo.KString {
				response["redirect_to"] = redirVal.Str
			}
		}
	}

	// If Filo returned an error, report it
	if errStr, ok := response["error"].(string); ok && errStr != "" {
		jsonResponse(w, http.StatusBadRequest, response)
		return
	}

	jsonResponse(w, http.StatusOK, response)
}

// MenuActionsJS serves dynamically generated JavaScript for menu item actions.
// GET /menu/{menuMachineName}/actions.js
func (h *Handlers) MenuActionsJS(w http.ResponseWriter, r *http.Request) {
	menuMachineName := r.PathValue("menuMachineName")

	// Get menu
	menu, err := db.Storage.GetMenuByMachineName(menuMachineName)
	if err != nil || menu == nil {
		http.Error(w, "// Menu not found", http.StatusNotFound)
		return
	}

	// Get menu items
	items, err := db.Storage.ListMenuItems(menu.ID)
	if err != nil {
		http.Error(w, "// Failed to load items", http.StatusInternalServerError)
		return
	}

	// Set content type as JavaScript
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Generate JavaScript
	var js strings.Builder
	js.WriteString("// Auto-generated menu actions for: ")
	js.WriteString(menu.MachineName)
	js.WriteString("\nwindow.MenuItemActions = {\n")

	first := true
	for _, item := range items {
		if item.JSCode != "" {
			if !first {
				js.WriteString(",\n")
			}
			first = false
			js.WriteString("    '")
			js.WriteString(item.MachineName)
			js.WriteString("': function() {\n        ")
			js.WriteString(item.JSCode)
			js.WriteString("\n    }")
		}
	}

	js.WriteString("\n};\n")

	_, _ = w.Write([]byte(js.String()))
}
