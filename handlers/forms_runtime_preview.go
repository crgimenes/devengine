package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/log"
)

// applyPrefill overrides initial values from query params of the shape
// `attr=value`. Each item in raw is one such pair; unknown attributes are
// ignored, and values are coerced into the primitive type expected by the
// attribute so the runtime template renders them correctly.
func applyPrefill(values map[string]any, attributes []db.EAVAttribute, raw []string) {
	if len(raw) == 0 {
		return
	}
	byName := make(map[string]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		byName[attributes[i].MachineName] = &attributes[i]
	}
	for _, pair := range raw {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || k == "" {
			continue
		}
		attr := byName[k]
		if attr == nil {
			continue
		}
		values[k] = coercePrefill(attr.PrimitiveKind, v)
	}
}

func coercePrefill(primitive, raw string) any {
	switch primitive {
	case "BOOL":
		return raw == "true" || raw == "1"
	case "INT":
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return raw
		}
		return n
	case "REAL":
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return raw
		}
		return f
	}
	return raw
}

// FormsRuntimePreview recomputes is_computed attributes with the values
// currently in the form and returns a JSON map of machine_name → value. The
// browser uses it to refresh computed inputs as the user types in their
// dependencies.
func (h *Handlers) FormsRuntimePreview(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, false, true,
	)
	if err != nil || !authed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	machineName := r.PathValue("machineName")
	form, err := db.Storage.GetFormByMachineName(machineName)
	if err != nil || form == nil || form.EAVEntityTypeID == nil {
		http.Error(w, "form not found", http.StatusNotFound)
		return
	}

	entityType, err := db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
	if err != nil {
		http.Error(w, "entity type not found", http.StatusInternalServerError)
		return
	}

	elements, err := db.Storage.ListFormElements(form.ID)
	if err != nil {
		http.Error(w, "failed to list form elements", http.StatusInternalServerError)
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(entityType.ID)
	if err != nil {
		http.Error(w, "failed to list attributes", http.StatusInternalServerError)
		return
	}

	values := parseFormValuesTolerant(r, elements, attributes)

	computed, err := applyComputedExprs(r.Context(), user, attributes, values)
	if err != nil {
		log.Printf("preview applyComputedExprs: %v", err)
		http.Error(w, "compute error", http.StatusInternalServerError)
		return
	}

	out := make(map[string]any, len(attributes))
	for i := range attributes {
		attr := &attributes[i]
		if !attr.IsComputed {
			continue
		}
		out[attr.MachineName] = computed[attr.MachineName]
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(out)
	if err != nil {
		log.Printf("preview encode: %v", err)
	}
}

// parseFormValuesTolerant parses submitted form values the same way
// parseFormAttributes does, but does NOT return errors for missing required
// fields. Used by the preview endpoint where partial input is expected.
func parseFormValuesTolerant(
	r *http.Request,
	elements []db.FormElement,
	attributes []db.EAVAttribute,
) db.EAVRecordValues {
	attrMap := make(map[int64]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		attrMap[attributes[i].ID] = &attributes[i]
	}

	out := make(db.EAVRecordValues)
	for _, el := range elements {
		if el.EAVAttributeID == nil {
			continue
		}
		attr := attrMap[*el.EAVAttributeID]
		if attr == nil {
			continue
		}
		raw := r.FormValue(el.MachineName)
		if raw == "" {
			out[attr.MachineName] = ""
			continue
		}
		v, err := parseElementValue(el, attr, raw)
		if err != nil {
			out[attr.MachineName] = raw
			continue
		}
		out[attr.MachineName] = v
	}
	return out
}
