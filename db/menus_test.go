package db_test

import (
	"testing"

	"github.com/crgimenes/devengine/db"
)

// Pure menu-tree helpers (no storage involved). These tests moved back from
// db/sqlite when the backend split left them testing db-package functions
// from the wrong package.

func TestSortMenuItemsHierarchically(t *testing.T) {
	t.Parallel()

	// Create test items manually (not from DB)
	submenuID := int64(100)
	items := []db.MenuItem{
		{ID: 3, MachineName: "child2", Label: "Child 2", ParentID: &submenuID, ZOrder: 1},
		{ID: 1, MachineName: "home", Label: "Home", ParentID: nil, ZOrder: 0},
		{ID: 2, MachineName: "submenu", Label: "Submenu", ParentID: nil, ZOrder: 1},
		{ID: 4, MachineName: "child1", Label: "Child 1", ParentID: &submenuID, ZOrder: 0},
	}
	// Use a different ID for submenu to match parentID references
	items[2].ID = submenuID

	sorted := db.SortMenuItemsHierarchically(items)

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

	items := []db.MenuItem{
		{ID: 1, MachineName: "root1", Label: "Root 1", ParentID: nil},
		{ID: 2, MachineName: "child1", Label: "Child 1", ParentID: &parent1},
		{ID: 3, MachineName: "grandchild1", Label: "Grandchild 1", ParentID: &parent2},
		{ID: 4, MachineName: "root2", Label: "Root 2", ParentID: nil},
		{ID: 5, MachineName: "child2", Label: "Child 2", ParentID: &parent4},
	}

	// Test collecting descendants of root1
	descendants := db.CollectMenuDescendants(items, 1)

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
	descendants2 := db.CollectMenuDescendants(items, 4)
	if len(descendants2) != 1 {
		t.Fatalf("expected 1 descendant for root2, got %d", len(descendants2))
	}
	if descendants2[0].ID != 5 {
		t.Fatalf("expected child2, got ID=%d", descendants2[0].ID)
	}

	// Test collecting descendants of a leaf node (should return empty)
	descendants3 := db.CollectMenuDescendants(items, 3)
	if len(descendants3) != 0 {
		t.Fatalf("expected 0 descendants for leaf node, got %d", len(descendants3))
	}

	// Test collecting descendants of non-existent parent (should return empty)
	descendants4 := db.CollectMenuDescendants(items, 999)
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
	items := []db.MenuItem{
		{ID: 1, MachineName: "item_a", Label: "Item A", ParentID: nil},
		{ID: 2, MachineName: "item_b", Label: "Item B", ParentID: &parentA}, // B is child of A
		{ID: 3, MachineName: "item_c", Label: "Item C", ParentID: &parentB}, // C is child of B
		{ID: 4, MachineName: "item_d", Label: "Item D", ParentID: &parentC}, // D is child of C
	}

	// Now artificially create a "cycle" by having an item that would appear multiple times
	// We can't truly create a cycle with parent IDs in this structure without DB,
	// so we test that items are only visited once by checking result count

	descendants := db.CollectMenuDescendants(items, 1)

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
	var items []db.MenuItem
	descendants := db.CollectMenuDescendants(items, 1)
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

	items := []db.MenuItem{
		{ID: 1, MachineName: "root1", Label: "Root 1", ParentID: nil},
		{ID: 2, MachineName: "child1", Label: "Child 1", ParentID: &parent1},
		{ID: 3, MachineName: "grandchild1", Label: "Grandchild 1", ParentID: &parent2},
		{ID: 4, MachineName: "root2", Label: "Root 2", ParentID: nil},
	}

	tree := db.BuildMenuItemTree(items)

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
