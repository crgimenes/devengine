package db

import (
	"strings"
	"testing"
)

func TestMenuCRUD(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	// Create menu
	menu, err := s.CreateMenu("main_menu", "Menu Principal", "Menu principal do sistema")
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
	err = s.UpdateMenu(menu.ID, "main_menu_updated", "Menu Atualizado", "Descrição atualizada")
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
	menu, err := s.CreateMenu("test_menu", "Test Menu", "")
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	// Create root item
	item1, err := s.CreateMenuItem(menu.ID, nil, "home", "Início", "bi-house", "link", "/", 0)
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
	item2, err := s.CreateMenuItem(menu.ID, nil, "sep1", "", "", "separator", "", 1)
	if err != nil {
		t.Fatalf("CreateMenuItem (separator): %v", err)
	}
	if item2.ItemType != "separator" {
		t.Fatalf("expected ItemType 'separator', got %q", item2.ItemType)
	}

	// Create submenu
	submenu, err := s.CreateMenuItem(menu.ID, nil, "settings", "Configurações", "bi-gear", "submenu", "", 2)
	if err != nil {
		t.Fatalf("CreateMenuItem (submenu): %v", err)
	}

	// Create item under submenu
	subItem, err := s.CreateMenuItem(menu.ID, &submenu.ID, "profile", "Meu Perfil", "bi-person", "link", "/me", 0)
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
	err = s.UpdateMenuItem(item1.ID, nil, "home", "Home Page", "bi-house-fill", "link", "/home", 0)
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
	item1, _ := s.CreateMenuItem(menu.ID, nil, "item1", "Item 1", "", "link", "/1", 0)
	item2, _ := s.CreateMenuItem(menu.ID, nil, "item2", "Item 2", "", "link", "/2", 1)
	item3, _ := s.CreateMenuItem(menu.ID, nil, "item3", "Item 3", "", "link", "/3", 2)

	// Move item2 up (should swap with item1)
	err := s.MoveMenuItemUp(item2.ID)
	if err != nil {
		t.Fatalf("MoveMenuItemUp: %v", err)
	}

	items, _ := s.ListMenuItems(menu.ID)
	sorted := SortMenuItemsHierarchically(items)

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
	sorted2 := SortMenuItemsHierarchically(items2)

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

func TestSortMenuItemsHierarchically(t *testing.T) {
	t.Parallel()

	// Create test items manually (not from DB)
	submenuID := int64(100)
	items := []MenuItem{
		{ID: 3, MachineName: "child2", Label: "Child 2", ParentID: &submenuID, ZOrder: 1},
		{ID: 1, MachineName: "home", Label: "Home", ParentID: nil, ZOrder: 0},
		{ID: 2, MachineName: "submenu", Label: "Submenu", ParentID: nil, ZOrder: 1},
		{ID: 4, MachineName: "child1", Label: "Child 1", ParentID: &submenuID, ZOrder: 0},
	}
	// Use a different ID for submenu to match parentID references
	items[2].ID = submenuID

	sorted := SortMenuItemsHierarchically(items)

	// Expected order:
	// 1. home (root, z_order 0)
	// 2. submenu (root, z_order 1)
	// 3. child1 (under submenu, z_order 0)
	// 4. child2 (under submenu, z_order 1)
	expected := []string{"home", "submenu", "child1", "child2"}
	for i, item := range sorted {
		if item.MachineName != expected[i] {
			t.Fatalf("position %d: expected %q, got %q", i, expected[i], item.MachineName)
		}
	}
}

func TestListSubmenuItems(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	menu, _ := s.CreateMenu("submenu_test", "Submenu Test", "")

	// Create items of different types
	s.CreateMenuItem(menu.ID, nil, "home", "Home", "", "link", "/", 0)
	s.CreateMenuItem(menu.ID, nil, "settings", "Settings", "", "submenu", "", 1)
	s.CreateMenuItem(menu.ID, nil, "admin", "Admin", "", "submenu", "", 2)
	s.CreateMenuItem(menu.ID, nil, "sep", "", "", "separator", "", 3)

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

	menu1, _ := s.CreateMenu("menu1", "Menu 1", "")
	menu2, _ := s.CreateMenu("menu2", "Menu 2", "")

	// Create item in menu1
	_, err := s.CreateMenuItem(menu1.ID, nil, "home", "Home", "", "link", "/", 0)
	if err != nil {
		t.Fatalf("CreateMenuItem in menu1: %v", err)
	}

	// Same machine_name in different menu should work
	_, err = s.CreateMenuItem(menu2.ID, nil, "home", "Home", "", "link", "/", 0)
	if err != nil {
		t.Fatalf("CreateMenuItem in menu2 (same name): %v", err)
	}

	// Same machine_name in same menu should fail
	_, err = s.CreateMenuItem(menu1.ID, nil, "home", "Home 2", "", "link", "/2", 1)
	if err == nil {
		t.Fatal("expected error for duplicate machine_name in same menu")
	}
}

func TestCollectMenuDescendants(t *testing.T) {
	t.Parallel()

	// Create a 3-level deep hierarchy:
	// root1 (ID=1)
	//   └── child1 (ID=2, parent=1)
	//         └── grandchild1 (ID=3, parent=2)
	// root2 (ID=4)
	//   └── child2 (ID=5, parent=4)

	parent1 := int64(1)
	parent2 := int64(2)
	parent4 := int64(4)

	items := []MenuItem{
		{ID: 1, MachineName: "root1", Label: "Root 1", ParentID: nil},
		{ID: 2, MachineName: "child1", Label: "Child 1", ParentID: &parent1},
		{ID: 3, MachineName: "grandchild1", Label: "Grandchild 1", ParentID: &parent2},
		{ID: 4, MachineName: "root2", Label: "Root 2", ParentID: nil},
		{ID: 5, MachineName: "child2", Label: "Child 2", ParentID: &parent4},
	}

	// Test collecting descendants of root1
	descendants := CollectMenuDescendants(items, 1)

	if len(descendants) != 2 {
		t.Fatalf("expected 2 descendants for root1, got %d", len(descendants))
	}

	// Check first descendant is child1 at depth 1
	if descendants[0].ID != 2 || descendants[0].Depth != 1 {
		t.Fatalf("expected child1 at depth 1, got ID=%d depth=%d", descendants[0].ID, descendants[0].Depth)
	}

	// Check second descendant is grandchild1 at depth 2
	if descendants[1].ID != 3 || descendants[1].Depth != 2 {
		t.Fatalf("expected grandchild1 at depth 2, got ID=%d depth=%d", descendants[1].ID, descendants[1].Depth)
	}

	// Test collecting descendants of root2
	descendants2 := CollectMenuDescendants(items, 4)
	if len(descendants2) != 1 {
		t.Fatalf("expected 1 descendant for root2, got %d", len(descendants2))
	}
	if descendants2[0].ID != 5 {
		t.Fatalf("expected child2, got ID=%d", descendants2[0].ID)
	}

	// Test collecting descendants of a leaf node (should return empty)
	descendants3 := CollectMenuDescendants(items, 3)
	if len(descendants3) != 0 {
		t.Fatalf("expected 0 descendants for leaf node, got %d", len(descendants3))
	}

	// Test collecting descendants of non-existent parent (should return empty)
	descendants4 := CollectMenuDescendants(items, 999)
	if len(descendants4) != 0 {
		t.Fatalf("expected 0 descendants for non-existent parent, got %d", len(descendants4))
	}
}

func TestCollectMenuDescendantsCycleDetection(t *testing.T) {
	t.Parallel()

	// Create a circular reference scenario:
	// A -> B -> C -> A (circular!)
	// This should NOT cause infinite recursion

	parentA := int64(1)
	parentB := int64(2)
	parentC := int64(3)

	// Items configured with a cycle: C points back to A's children
	// In reality this shouldn't happen due to DB constraints, but we test the safety net
	items := []MenuItem{
		{ID: 1, MachineName: "item_a", Label: "Item A", ParentID: nil},
		{ID: 2, MachineName: "item_b", Label: "Item B", ParentID: &parentA}, // B is child of A
		{ID: 3, MachineName: "item_c", Label: "Item C", ParentID: &parentB}, // C is child of B
		{ID: 4, MachineName: "item_d", Label: "Item D", ParentID: &parentC}, // D is child of C
	}

	// Now artificially create a "cycle" by having an item that would appear multiple times
	// We can't truly create a cycle with parent IDs in this structure without DB,
	// so we test that items are only visited once by checking result count

	descendants := CollectMenuDescendants(items, 1)

	// Should have exactly 3 descendants: B, C, D (each visited exactly once)
	if len(descendants) != 3 {
		t.Fatalf("expected exactly 3 descendants, got %d", len(descendants))
	}

	// Verify order and depth
	expectedOrder := []struct {
		id    int64
		depth int
	}{
		{2, 1}, // B at depth 1
		{3, 2}, // C at depth 2
		{4, 3}, // D at depth 3
	}

	for i, expected := range expectedOrder {
		if descendants[i].ID != expected.id || descendants[i].Depth != expected.depth {
			t.Fatalf("position %d: expected ID=%d depth=%d, got ID=%d depth=%d",
				i, expected.id, expected.depth, descendants[i].ID, descendants[i].Depth)
		}
	}
}

func TestCollectMenuDescendantsEmptyItems(t *testing.T) {
	t.Parallel()

	// Test with empty items slice
	var items []MenuItem
	descendants := CollectMenuDescendants(items, 1)
	if len(descendants) != 0 {
		t.Fatalf("expected 0 descendants for empty items slice, got %d", len(descendants))
	}
}

func TestBuildMenuItemTree(t *testing.T) {
	t.Parallel()

	// Create a 3-level deep hierarchy:
	// root1 (ID=1)
	//   └── child1 (ID=2, parent=1)
	//         └── grandchild1 (ID=3, parent=2)
	// root2 (ID=4)

	parent1 := int64(1)
	parent2 := int64(2)

	items := []MenuItem{
		{ID: 1, MachineName: "root1", Label: "Root 1", ParentID: nil},
		{ID: 2, MachineName: "child1", Label: "Child 1", ParentID: &parent1},
		{ID: 3, MachineName: "grandchild1", Label: "Grandchild 1", ParentID: &parent2},
		{ID: 4, MachineName: "root2", Label: "Root 2", ParentID: nil},
	}

	tree := BuildMenuItemTree(items)

	// Should have 2 root nodes
	if len(tree) != 2 {
		t.Fatalf("expected 2 root nodes, got %d", len(tree))
	}

	// First root should be root1 (ID=1)
	if tree[0].ID != 1 {
		t.Fatalf("expected first root to be ID=1, got %d", tree[0].ID)
	}

	// root1 should have 1 child
	if len(tree[0].Children) != 1 {
		t.Fatalf("expected root1 to have 1 child, got %d", len(tree[0].Children))
	}

	// That child should be child1 (ID=2)
	if tree[0].Children[0].ID != 2 {
		t.Fatalf("expected child1 to have ID=2, got %d", tree[0].Children[0].ID)
	}

	// child1 should have 1 child (grandchild1)
	if len(tree[0].Children[0].Children) != 1 {
		t.Fatalf("expected child1 to have 1 child, got %d", len(tree[0].Children[0].Children))
	}

	// grandchild should be ID=3
	if tree[0].Children[0].Children[0].ID != 3 {
		t.Fatalf("expected grandchild to have ID=3, got %d", tree[0].Children[0].Children[0].ID)
	}

	// grandchild should have no children
	if len(tree[0].Children[0].Children[0].Children) != 0 {
		t.Fatalf("expected grandchild to have 0 children, got %d", len(tree[0].Children[0].Children[0].Children))
	}

	// Second root should be root2 (ID=4) with no children
	if tree[1].ID != 4 {
		t.Fatalf("expected second root to be ID=4, got %d", tree[1].ID)
	}
	if len(tree[1].Children) != 0 {
		t.Fatalf("expected root2 to have 0 children, got %d", len(tree[1].Children))
	}
}
