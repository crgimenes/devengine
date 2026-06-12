package db

import (
	"time"
)

// Menu represents a menu definition.
type Menu struct {
	ID          int64
	ReferenceID string
	MachineName string
	Label       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// MenuItem represents a menu item.
type MenuItem struct {
	ID          int64
	ReferenceID string
	MenuID      int64
	ParentID    *int64 // NULL for root-level items
	MachineName string
	Label       string
	Icon        string // Optional Bootstrap icon class
	ItemType    string // 'link', 'separator', 'submenu'
	URL         string // Optional, for links
	JSCode      string // JavaScript code to execute client-side
	FiloCode    string // Filo code to execute server-side
	ZOrder      int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// SortMenuItemsHierarchically sorts items so that children appear immediately
// after their parent, respecting z_order within each level.
func SortMenuItemsHierarchically(items []MenuItem) []MenuItem {
	if len(items) == 0 {
		return items
	}

	// Build a map of parent_id -> children, sorted by z_order
	childrenOf := make(map[int64][]MenuItem)
	var roots []MenuItem

	for _, item := range items {
		if item.ParentID == nil {
			roots = append(roots, item)
		} else {
			childrenOf[*item.ParentID] = append(childrenOf[*item.ParentID], item)
		}
	}

	// Sort roots by z_order
	sortMenuItemsByZOrder(roots)

	// Sort each children group by z_order
	for parentID := range childrenOf {
		sortMenuItemsByZOrder(childrenOf[parentID])
	}

	// Recursively flatten: for each root, add it, then add all descendants
	var result []MenuItem
	var addWithChildren func(item MenuItem)
	addWithChildren = func(item MenuItem) {
		result = append(result, item)
		for _, child := range childrenOf[item.ID] {
			addWithChildren(child)
		}
	}

	for _, root := range roots {
		addWithChildren(root)
	}

	return result
}

// sortMenuItemsByZOrder sorts a slice of MenuItem by ZOrder in place.
func sortMenuItemsByZOrder(items []MenuItem) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].ZOrder < items[i].ZOrder ||
				(items[j].ZOrder == items[i].ZOrder && items[j].MachineName < items[i].MachineName) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// DescendantItem represents a menu item with its depth in the hierarchy.
type DescendantItem struct {
	MenuItem
	Depth int
}

// CollectMenuDescendants recursively collects all descendants of a given parent ID.
// It includes cycle detection: if an item ID is visited more than once, its children
// are not processed to prevent infinite recursion.
func CollectMenuDescendants(items []MenuItem, parentID int64) []DescendantItem {
	// Build a map of parent_id -> children
	childrenOf := make(map[int64][]MenuItem)
	for _, item := range items {
		if item.ParentID != nil {
			childrenOf[*item.ParentID] = append(childrenOf[*item.ParentID], item)
		}
	}

	// Track visited IDs to prevent cycles
	visited := make(map[int64]bool)

	var collect func(parentID int64, depth int) []DescendantItem
	collect = func(parentID int64, depth int) []DescendantItem {
		var result []DescendantItem
		for _, child := range childrenOf[parentID] {
			// Cycle detection: skip if already visited
			if visited[child.ID] {
				continue
			}
			visited[child.ID] = true

			result = append(result, DescendantItem{MenuItem: child, Depth: depth})
			// Recursively add grandchildren, etc.
			result = append(result, collect(child.ID, depth+1)...)
		}
		return result
	}

	return collect(parentID, 1)
}

// GetDirectChildren returns only the direct children of a parent ID from the items slice.
func GetDirectChildren(items []MenuItem, parentID int64) []MenuItem {
	var children []MenuItem
	for _, item := range items {
		if item.ParentID != nil && *item.ParentID == parentID {
			children = append(children, item)
		}
	}
	return children
}

// MenuItemNode represents a menu item with its children as a tree structure.
type MenuItemNode struct {
	MenuItem
	Children        []MenuItemNode
	MenuMachineName string // Menu machine name for action URLs
}

// BuildMenuItemTree builds a tree structure for menu items.
// It returns only the root-level items (items with no parent), each with their children populated recursively.
func BuildMenuItemTree(items []MenuItem) []MenuItemNode {
	return BuildMenuItemTreeWithName(items, "")
}

// BuildMenuItemTreeWithName builds a tree structure with the menu machine name propagated to all nodes.
func BuildMenuItemTreeWithName(items []MenuItem, menuMachineName string) []MenuItemNode {
	// Build a map of parent_id -> children
	childrenOf := make(map[int64][]MenuItem)
	for _, item := range items {
		if item.ParentID != nil {
			childrenOf[*item.ParentID] = append(childrenOf[*item.ParentID], item)
		}
	}

	// Track visited IDs to prevent cycles
	visited := make(map[int64]bool)

	var buildNode func(item MenuItem) MenuItemNode
	buildNode = func(item MenuItem) MenuItemNode {
		node := MenuItemNode{MenuItem: item, MenuMachineName: menuMachineName}

		// Cycle detection
		if visited[item.ID] {
			return node
		}
		visited[item.ID] = true

		// Add children recursively
		for _, child := range childrenOf[item.ID] {
			node.Children = append(node.Children, buildNode(child))
		}
		return node
	}

	// Build tree from root items only
	var roots []MenuItemNode
	for _, item := range items {
		if item.ParentID == nil {
			roots = append(roots, buildNode(item))
		}
	}
	return roots
}
