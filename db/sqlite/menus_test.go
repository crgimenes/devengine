package sqlite

import (
	"strings"
	"testing"

	"github.com/crgimenes/devengine/db"
)

func TestMenuCRUD(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	// Create menu
	menu, err := s.CreateMenu("main_menu", "db.Menu Principal", "db.Menu principal do sistema")
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	if menu.ID == 0 {
		t.Fatal("expected non-zero menu ID")
	}
	if menu.MachineName != "main_menu" {
		t.Fatalf("expected MachineName 'main_menu', got %q", menu.MachineName)
	}
	if menu.ReferenceID == "" {
		t.Fatal("expected non-empty ReferenceID")
	}

	// Get by RefID
	fetched, err := s.GetMenuByRefID(menu.ReferenceID)
	if err != nil {
		t.Fatalf("GetMenuByRefID: %v", err)
	}
	if fetched.ID != menu.ID {
		t.Fatalf("expected ID %d, got %d", menu.ID, fetched.ID)
	}

	// Get by MachineName
	fetched2, err := s.GetMenuByMachineName("main_menu")
	if err != nil {
		t.Fatalf("GetMenuByMachineName: %v", err)
	}
	if fetched2.ID != menu.ID {
		t.Fatalf("expected ID %d, got %d", menu.ID, fetched2.ID)
	}

	// Get non-existing by MachineName returns nil, nil
	notFound, err := s.GetMenuByMachineName("nonexistent")
	if err != nil {
		t.Fatalf("GetMenuByMachineName (not found): %v", err)
	}
	if notFound != nil {
		t.Fatalf("expected nil for non-existing menu, got %v", notFound)
	}

	// List menus
	menus, err := s.ListMenus()
	if err != nil {
		t.Fatalf("ListMenus: %v", err)
	}
	if len(menus) != 1 {
		t.Fatalf("expected 1 menu, got %d", len(menus))
	}

	// Update menu
	err = s.UpdateMenu(menu.ID, "main_menu_updated", "db.Menu Atualizado", "Descrição atualizada")
	if err != nil {
		t.Fatalf("UpdateMenu: %v", err)
	}
	updated, _ := s.GetMenuByRefID(menu.ReferenceID)
	if updated.MachineName != "main_menu_updated" {
		t.Fatalf("expected updated MachineName, got %q", updated.MachineName)
	}

	// Soft delete
	err = s.SoftDeleteMenu(menu.ID)
	if err != nil {
		t.Fatalf("SoftDeleteMenu: %v", err)
	}
	menusAfterDelete, _ := s.ListMenus()
	if len(menusAfterDelete) != 0 {
		t.Fatalf("expected 0 menus after soft delete, got %d", len(menusAfterDelete))
	}
}

func TestMenuItemCRUD(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	// Create menu first
	menu, err := s.CreateMenu("test_menu", "Test db.Menu", "")
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	// Create root item
	item1, err := s.CreateMenuItem(menu.ID, nil, "home", "Início", "bi-house", "link", "/", "", "", 0)
	if err != nil {
		t.Fatalf("CreateMenuItem: %v", err)
	}
	if item1.ID == 0 {
		t.Fatal("expected non-zero item ID")
	}
	if item1.ItemType != "link" {
		t.Fatalf("expected ItemType 'link', got %q", item1.ItemType)
	}

	// Create separator
	item2, err := s.CreateMenuItem(menu.ID, nil, "sep1", "", "", "separator", "", "", "", 1)
	if err != nil {
		t.Fatalf("CreateMenuItem (separator): %v", err)
	}
	if item2.ItemType != "separator" {
		t.Fatalf("expected ItemType 'separator', got %q", item2.ItemType)
	}

	// Create submenu
	submenu, err := s.CreateMenuItem(menu.ID, nil, "settings", "Configurações", "bi-gear", "submenu", "", "", "", 2)
	if err != nil {
		t.Fatalf("CreateMenuItem (submenu): %v", err)
	}

	// Create item under submenu
	subItem, err := s.CreateMenuItem(menu.ID, &submenu.ID, "profile", "Meu Perfil", "bi-person", "link", "/me", "", "", 0)
	if err != nil {
		t.Fatalf("CreateMenuItem (sub-item): %v", err)
	}
	if subItem.ParentID == nil || *subItem.ParentID != submenu.ID {
		t.Fatal("expected subItem to have submenu as parent")
	}

	// List items
	items, err := s.ListMenuItems(menu.ID)
	if err != nil {
		t.Fatalf("ListMenuItems: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}

	// Get by RefID
	fetched, err := s.GetMenuItemByRefID(item1.ReferenceID)
	if err != nil {
		t.Fatalf("GetMenuItemByRefID: %v", err)
	}
	if fetched.ID != item1.ID {
		t.Fatalf("expected ID %d, got %d", item1.ID, fetched.ID)
	}

	// Update item
	err = s.UpdateMenuItem(item1.ID, nil, "home", "Home Page", "bi-house-fill", "link", "/home", "", "", 0)
	if err != nil {
		t.Fatalf("UpdateMenuItem: %v", err)
	}
	updated, _ := s.GetMenuItemByRefID(item1.ReferenceID)
	if updated.Label != "Home Page" {
		t.Fatalf("expected updated Label 'Home Page', got %q", updated.Label)
	}

	// Delete item
	err = s.DeleteMenuItem(item2.ID)
	if err != nil {
		t.Fatalf("DeleteMenuItem: %v", err)
	}
	itemsAfterDelete, _ := s.ListMenuItems(menu.ID)
	if len(itemsAfterDelete) != 3 {
		t.Fatalf("expected 3 items after delete, got %d", len(itemsAfterDelete))
	}
}

func TestMenuItemMoveUpDown(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	menu, _ := s.CreateMenu("order_test", "Order Test", "")

	// Create items with explicit z_order
	item1, _ := s.CreateMenuItem(menu.ID, nil, "item1", "Item 1", "", "link", "/1", "", "", 0)
	item2, _ := s.CreateMenuItem(menu.ID, nil, "item2", "Item 2", "", "link", "/2", "", "", 1)
	item3, _ := s.CreateMenuItem(menu.ID, nil, "item3", "Item 3", "", "link", "/3", "", "", 2)

	// Move item2 up (should swap with item1)
	err := s.MoveMenuItemUp(item2.ID)
	if err != nil {
		t.Fatalf("MoveMenuItemUp: %v", err)
	}

	items, _ := s.ListMenuItems(menu.ID)
	sorted := db.SortMenuItemsHierarchically(items)

	// After move up: item2 should be first (z_order 0), item1 second (z_order 1)
	if sorted[0].ID != item2.ID {
		t.Fatalf("expected item2 first after move up, got %s", sorted[0].MachineName)
	}
	if sorted[1].ID != item1.ID {
		t.Fatalf("expected item1 second after move up, got %s", sorted[1].MachineName)
	}

	// Move item1 down (should swap with item3)
	err = s.MoveMenuItemDown(item1.ID)
	if err != nil {
		t.Fatalf("MoveMenuItemDown: %v", err)
	}

	items2, _ := s.ListMenuItems(menu.ID)
	sorted2 := db.SortMenuItemsHierarchically(items2)

	// After move down: order should be item2, item3, item1
	if sorted2[0].ID != item2.ID || sorted2[1].ID != item3.ID || sorted2[2].ID != item1.ID {
		t.Fatalf("unexpected order after move down: %s, %s, %s",
			sorted2[0].MachineName, sorted2[1].MachineName, sorted2[2].MachineName)
	}

	// Try to move first item up (should be no-op)
	err = s.MoveMenuItemUp(item2.ID)
	if err != nil {
		t.Fatalf("MoveMenuItemUp (already at top): %v", err)
	}

	// Try to move last item down (should be no-op)
	err = s.MoveMenuItemDown(item1.ID)
	if err != nil {
		t.Fatalf("MoveMenuItemDown (already at bottom): %v", err)
	}
}

func TestListSubmenuItems(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	menu, _ := s.CreateMenu("submenu_test", "Submenu Test", "")

	// Create items of different types
	for _, it := range []struct {
		machine, label, kind, url string
		order                     int
	}{
		{"home", "Home", "link", "/", 0},
		{"settings", "Settings", "submenu", "", 1},
		{"admin", "Admin", "submenu", "", 2},
		{"sep", "", "separator", "", 3},
	} {
		_, err := s.CreateMenuItem(menu.ID, nil, it.machine, it.label, "", it.kind, it.url, "", "", it.order)
		if err != nil {
			t.Fatalf("CreateMenuItem(%s): %v", it.machine, err)
		}
	}

	// List only submenus
	submenus, err := s.ListSubmenuItems(menu.ID)
	if err != nil {
		t.Fatalf("ListSubmenuItems: %v", err)
	}
	if len(submenus) != 2 {
		t.Fatalf("expected 2 submenus, got %d", len(submenus))
	}
	for _, sm := range submenus {
		if sm.ItemType != "submenu" {
			t.Fatalf("expected item_type 'submenu', got %q", sm.ItemType)
		}
	}
}

func TestMenuMachineNameUniqueness(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	// Create first menu
	_, err := s.CreateMenu("unique_test", "Unique Test", "")
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	// Try to create menu with same machine_name
	_, err = s.CreateMenu("unique_test", "Duplicate", "")
	if err == nil {
		t.Fatal("expected error for duplicate machine_name")
	}
	if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		t.Fatalf("expected UNIQUE constraint error, got: %v", err)
	}
}

func TestMenuItemMachineNameUniquenessPerMenu(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	menu1, _ := s.CreateMenu("menu1", "db.Menu 1", "")
	menu2, _ := s.CreateMenu("menu2", "db.Menu 2", "")

	// Create item in menu1
	_, err := s.CreateMenuItem(menu1.ID, nil, "home", "Home", "", "link", "/", "", "", 0)
	if err != nil {
		t.Fatalf("CreateMenuItem in menu1: %v", err)
	}

	// Same machine_name in different menu should work
	_, err = s.CreateMenuItem(menu2.ID, nil, "home", "Home", "", "link", "/", "", "", 0)
	if err != nil {
		t.Fatalf("CreateMenuItem in menu2 (same name): %v", err)
	}

	// Same machine_name in same menu should fail
	_, err = s.CreateMenuItem(menu1.ID, nil, "home", "Home 2", "", "link", "/2", "", "", 1)
	if err == nil {
		t.Fatal("expected error for duplicate machine_name in same menu")
	}
}
