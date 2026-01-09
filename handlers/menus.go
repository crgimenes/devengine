package handlers

import (
	"net/http"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/session"
)

// ToolsMenus shows the menu list page.
func (h *Handlers) ToolsMenus(w http.ResponseWriter, r *http.Request) {
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

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	message := r.URL.Query().Get("message")
	if len(message) > 200 {
		http.Error(w, "message too long", http.StatusBadRequest)
		return
	}

	menus, err := db.Storage.ListMenus()
	if err != nil {
		http.Error(w, "failed to fetch menus", http.StatusInternalServerError)
		return
	}

	data := struct {
		Authed      bool
		User        db.User
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Menus       []db.Menu
	}{
		Authed:      true,
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "menu-editor",
		Menus:       menus,
	}

	err = h.templates(w, "tools_menu_editor.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// ToolsMenusNew shows the new menu creation page.
func (h *Handlers) ToolsMenusNew(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	csrf := session.GenerateCSRFToken(w, r)

	data := struct {
		Authed      bool
		User        db.User
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Csrf        string
		FormData    struct{ MachineName, Label, Description string }
	}{
		Authed:      true,
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "menu-editor",
		Csrf:        csrf,
		FormData:    struct{ MachineName, Label, Description string }{},
	}

	err = h.templates(w, "tools_menu_editor_new.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// ToolsMenusCreate handles menu creation.
func (h *Handlers) ToolsMenusCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if !session.ValidateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
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
			Error:       "Nome e Nome da Máquina são obrigatórios",
			Config:      *h.cfg,
			CurrentPage: "menu-editor",
			Csrf:        csrf,
			FormData:    struct{ MachineName, Label, Description string }{machineName, label, description},
		}
		h.templates(w, "tools_menu_editor_new.go.tmpl", data)
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
			Error:       "Erro ao criar menu: " + err.Error(),
			Config:      *h.cfg,
			CurrentPage: "menu-editor",
			Csrf:        csrf,
			FormData:    struct{ MachineName, Label, Description string }{machineName, label, description},
		}
		h.templates(w, "tools_menu_editor_new.go.tmpl", data)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor/"+menu.ReferenceID+"/edit", http.StatusSeeOther)
}

// ToolsMenusEdit shows the menu edit page with items list.
func (h *Handlers) ToolsMenusEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing menu ID", http.StatusBadRequest)
		return
	}

	menu, err := db.Storage.GetMenuByRefID(id)
	if err != nil {
		http.Error(w, "menu not found", http.StatusNotFound)
		return
	}

	items, err := db.Storage.ListMenuItems(menu.ID)
	if err != nil {
		http.Error(w, "failed to fetch items", http.StatusInternalServerError)
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
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "menu-editor",
		Menu:        menu,
		Items:       itemViews,
		AllItems:    items,
		Csrf:        csrf,
	}

	err = h.templates(w, "tools_menu_editor_edit.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// ToolsMenusUpdate handles menu update.
func (h *Handlers) ToolsMenusUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if !session.ValidateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	id := r.PathValue("id")
	menu, err := db.Storage.GetMenuByRefID(id)
	if err != nil {
		http.Error(w, "menu not found", http.StatusNotFound)
		return
	}

	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	description := r.FormValue("description")

	err = db.Storage.UpdateMenu(menu.ID, machineName, label, description)
	if err != nil {
		http.Redirect(w, r, "/tools/menu-editor/"+id+"/edit?message=Erro+ao+atualizar", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor/"+id+"/edit?message=Menu+atualizado+com+sucesso", http.StatusSeeOther)
}

// ToolsMenusDelete handles menu deletion.
func (h *Handlers) ToolsMenusDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id := r.PathValue("id")
	menu, err := db.Storage.GetMenuByRefID(id)
	if err != nil {
		http.Error(w, "menu not found", http.StatusNotFound)
		return
	}

	err = db.Storage.SoftDeleteMenu(menu.ID)
	if err != nil {
		http.Redirect(w, r, "/tools/menu-editor?message=Erro+ao+excluir+menu", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor?message=Menu+excluído+com+sucesso", http.StatusSeeOther)
}

// ToolsMenusItemCreate handles creating a new menu item.
func (h *Handlers) ToolsMenusItemCreate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if !session.ValidateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	id := r.PathValue("id")
	menu, err := db.Storage.GetMenuByRefID(id)
	if err != nil {
		http.Error(w, "menu not found", http.StatusNotFound)
		return
	}

	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	icon := r.FormValue("icon")

	if machineName == "" {
		http.Redirect(w, r, "/tools/menu-editor/"+id+"/edit?message=Nome+da+máquina+é+obrigatório", http.StatusSeeOther)
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

	_, err = db.Storage.CreateMenuItem(menu.ID, parentID, machineName, label, icon, "link", "", maxZOrder)
	if err != nil {
		http.Redirect(w, r, "/tools/menu-editor/"+id+"/edit?message=Erro+ao+criar+item:+"+err.Error(), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor/"+id+"/edit?message=Item+criado+com+sucesso", http.StatusSeeOther)
}

// ToolsMenusItemEdit shows the item edit page.
func (h *Handlers) ToolsMenusItemEdit(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	menuID := r.PathValue("id")
	itemID := r.PathValue("item_id")

	menu, err := db.Storage.GetMenuByRefID(menuID)
	if err != nil {
		http.Error(w, "menu not found", http.StatusNotFound)
		return
	}

	item, err := db.Storage.GetMenuItemByRefID(itemID)
	if err != nil {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	// Get all items for parent selection
	allItems, _ := db.Storage.ListMenuItems(menu.ID)

	csrf := session.GenerateCSRFToken(w, r)
	message := r.URL.Query().Get("message")

	data := struct {
		Authed      bool
		User        db.User
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
		User:        *user,
		Message:     message,
		Config:      *h.cfg,
		CurrentPage: "menu-editor",
		Menu:        menu,
		Item:        item,
		AllItems:    allItems,
		Csrf:        csrf,
	}

	err = h.templates(w, "tools_menu_editor_item_edit.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// ToolsMenusItemUpdate handles item update.
func (h *Handlers) ToolsMenusItemUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if !session.ValidateCSRF(r) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}

	menuID := r.PathValue("id")
	itemID := r.PathValue("item_id")

	item, err := db.Storage.GetMenuItemByRefID(itemID)
	if err != nil {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	machineName := r.FormValue("machine_name")
	label := r.FormValue("label")
	icon := r.FormValue("icon")
	itemType := r.FormValue("item_type")
	url := r.FormValue("url")

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

	err = db.Storage.UpdateMenuItem(item.ID, parentID, machineName, label, icon, itemType, url, item.ZOrder)
	if err != nil {
		http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/items/"+itemID+"/edit?message=Erro+ao+atualizar", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/edit?message=Item+atualizado+com+sucesso", http.StatusSeeOther)
}

// ToolsMenusItemDelete handles item deletion.
func (h *Handlers) ToolsMenusItemDelete(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	menuID := r.PathValue("id")
	itemID := r.PathValue("item_id")

	item, err := db.Storage.GetMenuItemByRefID(itemID)
	if err != nil {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	err = db.Storage.DeleteMenuItem(item.ID)
	if err != nil {
		http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/edit?message=Erro+ao+excluir+item", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/edit?message=Item+excluído+com+sucesso", http.StatusSeeOther)
}

// ToolsMenusItemMoveUp moves an item up.
func (h *Handlers) ToolsMenusItemMoveUp(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	menuID := r.PathValue("id")
	itemID := r.PathValue("item_id")

	item, err := db.Storage.GetMenuItemByRefID(itemID)
	if err != nil {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	_ = db.Storage.MoveMenuItemUp(item.ID)

	http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/edit", http.StatusSeeOther)
}

// ToolsMenusItemMoveDown moves an item down.
func (h *Handlers) ToolsMenusItemMoveDown(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	menuID := r.PathValue("id")
	itemID := r.PathValue("item_id")

	item, err := db.Storage.GetMenuItemByRefID(itemID)
	if err != nil {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	_ = db.Storage.MoveMenuItemDown(item.ID)

	http.Redirect(w, r, "/tools/menu-editor/"+menuID+"/edit", http.StatusSeeOther)
}

// ToolsMenusPreview shows a preview of the menu.
func (h *Handlers) ToolsMenusPreview(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, false, true,
	)
	if err != nil || !authed {
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if !user.Sysop {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id := r.PathValue("id")
	menu, err := db.Storage.GetMenuByRefID(id)
	if err != nil {
		http.Error(w, "menu not found", http.StatusNotFound)
		return
	}

	items, err := db.Storage.ListMenuItems(menu.ID)
	if err != nil {
		http.Error(w, "failed to fetch items", http.StatusInternalServerError)
		return
	}

	// Build menu tree for the template using the db package function
	menuTree := db.BuildMenuItemTree(items)

	data := struct {
		Authed      bool
		User        db.User
		Config      config.Config
		CurrentPage string
		Menu        *db.Menu
		Items       []db.MenuItemNode
		AllItems    []db.MenuItem
	}{
		Authed:      true,
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "menu-editor",
		Menu:        menu,
		Items:       menuTree,
		AllItems:    items,
	}

	err = h.templates(w, "tools_menu_editor_preview.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
