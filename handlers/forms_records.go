package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/log"
)

const recordsBatchSize = 50

// RecordRow is the shape passed to the rows template: the record itself,
// plus a map of machine_name → display value already resolved per primitive.
type RecordRow struct {
	Record db.EAVRecord
	Values map[string]any
}

// ToolsFormsRecordsRows renders only the table rows + next sentinel. Used by
// HTMX for the infinite scroll. The full page (`ToolsFormsRecords`) renders
// the chrome and reuses this same data path for the initial batch.
func (h *Handlers) ToolsFormsRecordsRows(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	form, et, err := loadFormAndEntity(r, w)
	if err != nil || form == nil {
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil {
		log.Printf("ListEAVAttributesByEntityTypeID: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	rows, nextCursor, err := h.fetchRecordRows(et.ID, cursor, q, attributes)
	if err != nil {
		log.Printf("fetchRecordRows: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	resolveReferenceDisplays(form, attributes, rows)

	data := struct {
		FormRefID       string
		EntityTypeRefID string
		Attributes      []db.EAVAttribute
		Rows            []RecordRow
		NextCursor      int64
		Filter          string
		ColCount        int
	}{
		FormRefID:       form.ReferenceID,
		EntityTypeRefID: et.ReferenceID,
		Attributes:      attributes,
		Rows:            rows,
		NextCursor:      nextCursor,
		Filter:          q,
		ColCount:        len(attributes) + 2,
	}

	// The file is define-only; executing it by filename renders nothing, so
	// address the defined template directly.
	h.render(w, "forms_records_rows", data)
}

// ToolsFormsRecordsExport streams every record of the form's entity_type as
// CSV. Cursor + filter are ignored: this is a full dump.
func (h *Handlers) ToolsFormsRecordsExport(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil || !authed || !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	form, et, err := loadFormAndEntity(r, w)
	if err != nil || form == nil {
		return
	}

	attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil {
		log.Printf("ListEAVAttributesByEntityTypeID: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	filename := et.MachineName + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Reference columns export the display label in the main column plus a
	// "<name>_ref" column carrying the raw record id, so spreadsheets read
	// naturally and integrations keep the stable key.
	refLabels := referenceLabelMaps(form, attributes)

	header := []string{"reference_id", "created_at"}
	for _, a := range attributes {
		header = append(header, a.MachineName)
		if _, ok := refLabels[a.MachineName]; ok {
			header = append(header, a.MachineName+"_ref")
		}
	}
	err = cw.Write(header)
	if err != nil {
		log.Printf("csv write header: %v", err)
		return
	}

	var cursor int64
	for {
		rows, next, err := h.fetchRecordRows(et.ID, cursor, "", attributes)
		if err != nil {
			log.Printf("export fetchRecordRows: %v", err)
			return
		}
		if len(rows) == 0 {
			return
		}
		for _, row := range rows {
			line := []string{row.Record.ReferenceID, row.Record.CreatedAt.Format("2006-01-02T15:04:05Z07:00")}
			for _, a := range attributes {
				raw := formatValue(row.Values[a.MachineName])
				byValue, isRef := refLabels[a.MachineName]
				if !isRef {
					line = append(line, raw)
					continue
				}
				display := raw
				if label, ok := byValue[raw]; ok {
					display = label
				}
				line = append(line, display, raw)
			}
			err := cw.Write(line)
			if err != nil {
				log.Printf("csv write row: %v", err)
				return
			}
		}
		if next == 0 {
			return
		}
		cursor = next
	}
}

// fetchRecordRows resolves a single batch: cursor query → batched values →
// flattened map. Returns the rows and the cursor to use for the next batch
// (0 when there are no more records).
func (h *Handlers) fetchRecordRows(
	entityTypeID int64,
	cursor int64,
	filter string,
	attributes []db.EAVAttribute,
) ([]RecordRow, int64, error) {
	records, err := db.Storage.ListEAVRecordsCursor(entityTypeID, cursor, recordsBatchSize, filter)
	if err != nil {
		return nil, 0, err
	}
	if len(records) == 0 {
		return nil, 0, nil
	}

	ids := make([]int64, len(records))
	for i, r := range records {
		ids[i] = r.ID
	}
	valsByRecord, err := db.Storage.GetEAVValuesForRecordIDs(ids)
	if err != nil {
		return nil, 0, err
	}

	attrByID := make(map[int64]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		attrByID[attributes[i].ID] = &attributes[i]
	}

	rows := make([]RecordRow, len(records))
	for i, rec := range records {
		valMap := make(map[string]any, len(attributes))
		for _, v := range valsByRecord[rec.ID] {
			a := attrByID[v.AttributeID]
			if a == nil {
				continue
			}
			valMap[a.MachineName] = unwrapValue(a.PrimitiveKind, v)
		}
		rows[i] = RecordRow{Record: rec, Values: valMap}
	}

	nextCursor := int64(0)
	if len(records) == recordsBatchSize {
		nextCursor = records[len(records)-1].ID
	}
	return rows, nextCursor, nil
}

func loadFormAndEntity(r *http.Request, w http.ResponseWriter) (*db.Form, *db.EAVEntityType, error) {
	formRefID := r.PathValue("id")
	form, err := db.Storage.GetFormByRefID(formRefID)
	if err != nil {
		http.Error(w, "Form not found", http.StatusNotFound)
		return nil, nil, err
	}
	if form.EAVEntityTypeID == nil {
		http.Redirect(w, r,
			"/tools/forms/"+formRefID+"/edit?message="+i18n.T("Form has no linked EAV table"),
			http.StatusSeeOther)
		return nil, nil, fmt.Errorf("no entity type")
	}
	et, err := db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
	if err != nil {
		http.Error(w, "Entity type not found", http.StatusInternalServerError)
		return form, nil, err
	}
	return form, et, nil
}

// unwrapValue picks the right typed column based on the attribute's primitive
// kind and returns a Go value the template can render directly.
func unwrapValue(primitive string, v db.EAVValue) any {
	switch primitive {
	case "BOOL":
		if v.VBool != nil {
			return *v.VBool
		}
	case "INT":
		if v.VInt != nil {
			return *v.VInt
		}
	case "REAL":
		if v.VReal != nil {
			return *v.VReal
		}
	case "TEXT":
		if v.VText != nil {
			return *v.VText
		}
	case "DATETIME":
		if v.VDatetime != nil {
			return *v.VDatetime
		}
	}
	return nil
}

func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case string:
		return x
	}
	return fmt.Sprintf("%v", v)
}
