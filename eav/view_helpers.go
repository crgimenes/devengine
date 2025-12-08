package eav

import (
	"sort"

	"github.com/crgimenes/devengine/db"
)

// filterFieldsByViewMode filters fields based on visibility for the given view mode
func filterFieldsByViewMode(fields []db.EAVField, mode string) []db.EAVField {
	if mode == "" {
		mode = "list"
	}

	visible := make([]db.EAVField, 0, len(fields))
	for _, f := range fields {
		var isVisible bool
		switch mode {
		case "list":
			isVisible = f.ListVisible
		case "card":
			isVisible = f.CardVisible
		case "carousel":
			isVisible = f.CarouselVisible
		default:
			isVisible = f.ListVisible
		}

		if isVisible {
			visible = append(visible, f)
		}
	}

	return visible
}

// sortFieldsByViewMode sorts fields by z_order for the given view mode
func sortFieldsByViewMode(fields []db.EAVField, mode string) {
	if mode == "" {
		mode = "list"
	}

	sort.SliceStable(fields, func(i, j int) bool {
		var orderI, orderJ int
		switch mode {
		case "list":
			orderI, orderJ = fields[i].ListZOrder, fields[j].ListZOrder
		case "card":
			orderI, orderJ = fields[i].CardZOrder, fields[j].CardZOrder
		case "carousel":
			orderI, orderJ = fields[i].CarouselZOrder, fields[j].CarouselZOrder
		default:
			orderI, orderJ = fields[i].ListZOrder, fields[j].ListZOrder
		}

		// Primary sort by z_order
		if orderI != orderJ {
			return orderI < orderJ
		}

		// Secondary sort by field ID for stable ordering
		return fields[i].ID < fields[j].ID
	})
}

// getFieldColumnWidth returns the column width for a field in the given view mode
func getFieldColumnWidth(field db.EAVField, mode string) int {
	var width int
	switch mode {
	case "list":
		width = field.ListCols
	case "card":
		width = field.CardCols
	case "carousel":
		width = field.CarouselCols
	default:
		width = field.ListCols
	}

	// Ensure valid Bootstrap grid range (1-12)
	if width < 1 || width > 12 {
		return 12
	}
	return width
}

// normalizeViewMode returns a valid view mode or defaults to "list"
func normalizeViewMode(mode string) string {
	switch mode {
	case "list", "card", "carousel":
		return mode
	default:
		return "list"
	}
}
