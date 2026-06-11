package handlers

import (
	"net/http"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
)

// Each entity type shows at most this many matches; one extra is fetched to
// know whether a "see all" link is worth showing.
const searchGroupLimit = 5

// SearchHit is one matching record with a human summary.
type SearchHit struct {
	RefID   string
	Summary string
}

// SearchGroup carries the matches of one entity type. FormRefID points at the
// first form bound to the entity, whose records viewer supports the same
// text filter; empty when no form exists.
type SearchGroup struct {
	EntityType db.EAVEntityType
	FormRefID  string
	Hits       []SearchHit
	HasMore    bool
}

// resolveReferenceValues swaps raw reference ids for display labels inside a
// values map, using the bound form's element metadata. No form, no swap —
// the raw ids stay readable enough.
func resolveReferenceValues(form *db.Form, attributes []db.EAVAttribute, values map[string]any) {
	if form == nil {
		return
	}
	for machineName, byValue := range referenceLabelMaps(form, attributes) {
		v, ok := values[machineName].(string)
		if !ok {
			continue
		}
		label, ok := byValue[v]
		if ok {
			values[machineName] = label
		}
	}
}

// ToolsSearchForms is the global record search: one query matched against
// the TEXT values of every entity type, grouped by entity. sysop-only.
func (h *Handlers) ToolsSearchForms(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "ToolsSearchForms", err)
		return
	}
	if !authed {
		return
	}
	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 100 {
		query = query[:100]
	}

	var groups []SearchGroup
	searched := query != ""
	if searched {
		groups, err = h.searchAllEntities(query)
		if err != nil {
			h.serverError(w, r, "searchAllEntities", err)
			return
		}
	}

	data := struct {
		Authed      bool
		User        db.User
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Query       string
		Searched    bool
		Groups      []SearchGroup
	}{
		Authed:      true,
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "search-forms",
		Query:       query,
		Searched:    searched,
		Groups:      groups,
	}

	h.render(w, "tools_search_forms.go.tmpl", data)
}

// searchAllEntities runs the text filter over every entity type and builds
// the grouped result list. Entity types without matches are omitted.
func (h *Handlers) searchAllEntities(query string) ([]SearchGroup, error) {
	entityTypes, err := db.Storage.ListEAVEntityTypes()
	if err != nil {
		return nil, err
	}
	formByEntity, err := firstFormByEntityType()
	if err != nil {
		return nil, err
	}

	var groups []SearchGroup
	for _, et := range entityTypes {
		records, err := db.Storage.ListEAVRecordsCursor(et.ID, 0, searchGroupLimit+1, query)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			continue
		}
		hasMore := len(records) > searchGroupLimit
		if hasMore {
			records = records[:searchGroupLimit]
		}

		attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(et.ID)
		if err != nil {
			return nil, err
		}
		ids := make([]int64, len(records))
		for i, rec := range records {
			ids[i] = rec.ID
		}
		valuesByRecord, err := db.Storage.GetEAVValuesForRecordIDs(ids)
		if err != nil {
			return nil, err
		}

		form := formByEntity[et.ID]
		group := SearchGroup{
			EntityType: et,
			HasMore:    hasMore,
		}
		if form != nil {
			group.FormRefID = form.ReferenceID
		}
		for _, rec := range records {
			values := recordValuesMap(valuesByRecord[rec.ID], attributes)
			resolveReferenceValues(form, attributes, values)
			group.Hits = append(group.Hits, SearchHit{
				RefID:   rec.ReferenceID,
				Summary: summarizeRecord(attributes, values),
			})
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// firstFormByEntityType maps each entity type id to the first form bound to
// it.
func firstFormByEntityType() (map[int64]*db.Form, error) {
	forms, err := db.Storage.ListForms()
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*db.Form, len(forms))
	for i := range forms {
		f := &forms[i]
		if f.EAVEntityTypeID == nil {
			continue
		}
		if _, ok := out[*f.EAVEntityTypeID]; !ok {
			out[*f.EAVEntityTypeID] = f
		}
	}
	return out, nil
}

// summarizeRecord joins up to three "Label: value" pairs in attribute order.
func summarizeRecord(attributes []db.EAVAttribute, values map[string]any) string {
	var parts []string
	for _, a := range attributes {
		v, ok := values[a.MachineName]
		if !ok {
			continue
		}
		s := formatValue(v)
		if s == "" {
			continue
		}
		if len(s) > 60 {
			s = s[:60] + "…"
		}
		parts = append(parts, a.Label+": "+s)
		if len(parts) == 3 {
			break
		}
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " · ")
}
