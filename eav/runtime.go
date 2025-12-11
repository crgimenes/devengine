package eav

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/eav/ui"
	"github.com/crgimenes/devengine/log"
	"github.com/crgimenes/devengine/permit"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/devengine/templates"
)

const (
	eavPermView   = "eav.records.view"
	eavPermCreate = "eav.records.create"
	eavPermEdit   = "eav.records.edit"
	eavPermDelete = "eav.records.delete"
)

type runtimeFormContext struct {
	Workspace *db.EAVWorkspace
	Form      *db.EAVForm
	Fields    []db.EAVField
}

type runtimeFieldState struct {
	Field        db.EAVField
	InputName    string
	ColumnWidth  int
	Value        string
	BoolValue    bool
	Error        string
	IsUIOnly     bool
	ReadOnly     bool
	HasValue     bool
	ValuePayload runtimeValuePayload
	DisplayOnly  bool

	ValueAny     any
	RenderedEdit template.HTML
	RenderedView template.HTML
}

type runtimeFieldNode struct {
	State    *runtimeFieldState
	Children []*runtimeFieldNode
}

func (s *runtimeFieldState) applyExistingValue(val db.EAVValue) {
	if !valueHasContent(val) {
		return
	}
	s.HasValue = true
	switch s.Field.PrimitiveKind {
	case "TEXT":
		if val.ValueText != nil {
			s.Value = *val.ValueText
			s.ValueAny = *val.ValueText
		}
	case "INT":
		if val.ValueInt != nil {
			s.Value = strconv.FormatInt(*val.ValueInt, 10)
			s.ValueAny = *val.ValueInt
		}
	case "FLOAT":
		if val.ValueFloat != nil {
			s.Value = strconv.FormatFloat(*val.ValueFloat, 'f', -1, 64)
			s.ValueAny = *val.ValueFloat
		}
	case "DATETIME":
		if val.ValueDatetime != nil {
			s.Value = formatDatetimeForInput(*val.ValueDatetime)
			s.ValueAny = *val.ValueDatetime
		}
	case "BOOL":
		if val.ValueBool != nil {
			s.BoolValue = *val.ValueBool
			s.ValueAny = *val.ValueBool
		}
	}
}

type runtimeValuePayload struct {
	BoolValue    *bool
	TimeValue    *time.Time
	FloatValue   *float64
	IntValue     *int64
	TextValue    *string
	HasPayload   bool
	ShouldDelete bool
}

func runtimeScopeForForm(f *db.EAVForm) string {
	return fmt.Sprintf("form:%s", f.ReferenceID)
}

func loadRuntimeContext(workspaceRef, formMachineName string) (*runtimeFormContext, error) {
	ws, err := db.Storage.GetEAVWorkspaceByReferenceID(workspaceRef)
	if err != nil {
		return nil, err
	}

	form, err := db.Storage.GetEAVFormByMachineName(ws.ID, formMachineName)
	if err != nil {
		return nil, err
	}

	fields, err := db.Storage.GetEAVFieldsByFormID(form.ID)
	if err != nil {
		return nil, err
	}

	return &runtimeFormContext{
		Workspace: ws,
		Form:      form,
		Fields:    fields,
	}, nil
}

func (ctx *runtimeFormContext) renderableFields() []db.EAVField {
	out := make([]db.EAVField, 0, len(ctx.Fields))
	for _, f := range ctx.Fields {
		if shouldSuppressRuntimeField(f) {
			continue
		}
		out = append(out, f)
	}
	return out
}

func ensureRuntimePermission(w http.ResponseWriter, form *db.EAVForm, user *db.User, resource string) bool {
	if user.ID == form.OwnerUserID {
		return true
	}

	allowed, err := permit.Check(form.WorkspaceID, user.ID, resource, runtimeScopeForForm(form))
	if err != nil {
		log.Printf("permit check error: %v", err)
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return false
	}
	if !allowed {
		http.Error(w, "acesso negado", http.StatusForbidden)
		return false
	}
	return true
}

func parsePositiveInt(q url.Values, key string, def, max int) int {
	raw := q.Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return def
	}
	if max > 0 && v > max {
		return max
	}
	return v
}

func runtimeFieldInputName(id int64) string {
	return fmt.Sprintf("field_%d", id)
}

func normalizeColumnWidth(width int) int {
	if width < 1 || width > 12 {
		return 12
	}
	return width
}

func submittedValue(r *http.Request, key string) (string, bool) {
	vals, ok := r.Form[key]
	if !ok || len(vals) == 0 {
		return "", false
	}
	return vals[len(vals)-1], true
}

func formatDatetimeForInput(t time.Time) string {
	return t.In(time.Local).Format("2006-01-02T15:04")
}

func parseDatetimeInput(raw string) (*time.Time, error) {
	layouts := []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04"}
	for _, layout := range layouts {
		var (
			parsed time.Time
			err    error
		)
		if layout == time.RFC3339 {
			parsed, err = time.Parse(layout, raw)
		} else {
			parsed, err = time.ParseInLocation(layout, raw, time.Local)
			parsed = parsed.UTC()
		}
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, errors.New("invalid datetime")
}

func valueHasContent(v db.EAVValue) bool {
	return v.ValueBool != nil || v.ValueDatetime != nil || v.ValueFloat != nil || v.ValueInt != nil || v.ValueText != nil
}

func valuesByField(values []db.EAVValue) map[int64]db.EAVValue {
	out := make(map[int64]db.EAVValue, len(values))
	for _, v := range values {
		out[v.FieldID] = v
	}
	return out
}

func buildAllValues(states []runtimeFieldState) map[string]any {
	out := make(map[string]any, len(states))
	for _, st := range states {
		out[st.Field.MachineName] = st.ValueAny
	}
	return out
}

func isRuntimeGroupingField(field *db.EAVField) bool {
	if field == nil {
		return false
	}

	return field.IsGroupingField
}

type accordionMeta struct {
	StartOpen         bool   `json:"start_open,omitempty"`
	AccordionGroupKey string `json:"accordion_group_key,omitempty"`
}

func accordionGroupKey(meta string) string {
	opts := accordionMeta{}
	err := json.Unmarshal([]byte(meta), &opts)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(opts.AccordionGroupKey)
}

func sanitizeKeyForID(value string) string {
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func accordionContainerID(key string, fieldID int64) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return fmt.Sprintf("accordion-field-%d", fieldID)
	}
	safe := sanitizeKeyForID(trimmed)
	if safe == "" {
		return fmt.Sprintf("accordion-field-%d", fieldID)
	}
	return fmt.Sprintf("accordion-%s", safe)
}

func buildParentsMap(states []runtimeFieldState) map[string]string {
	parents := make(map[string]string, len(states))
	for _, st := range states {
		parents[st.Field.MachineName] = strings.TrimSpace(st.Field.ParentGroupMachineName)
	}
	return parents
}

func createsCycle(target, parent string, parents map[string]string) bool {
	current := parent
	visited := make(map[string]struct{})
	for current != "" {
		if current == target {
			return true
		}
		if _, ok := visited[current]; ok {
			return true
		}
		visited[current] = struct{}{}
		current = parents[current]
	}
	return false
}

func buildFieldTree(states []runtimeFieldState) []*runtimeFieldNode {
	parents := buildParentsMap(states)
	nodes := make(map[string]*runtimeFieldNode, len(states))
	for idx := range states {
		st := &states[idx]
		nodes[st.Field.MachineName] = &runtimeFieldNode{State: st}
	}

	roots := make([]*runtimeFieldNode, 0, len(states))
	for idx := range states {
		st := &states[idx]
		node := nodes[st.Field.MachineName]
		parentName := parents[st.Field.MachineName]
		if parentName == "" {
			roots = append(roots, node)
			continue
		}

		parentNode, ok := nodes[parentName]
		if !ok || !isRuntimeGroupingField(&parentNode.State.Field) || parentName == st.Field.MachineName {
			roots = append(roots, node)
			continue
		}

		if createsCycle(st.Field.MachineName, parentName, parents) {
			roots = append(roots, node)
			continue
		}

		parentNode.Children = append(parentNode.Children, node)
	}

	return roots
}

func applyValueToPayload(st *runtimeFieldState, isEdit bool) string {
	if st.IsUIOnly || st.ReadOnly {
		return ""
	}

	switch st.Field.PrimitiveKind {
	case "BOOL":
		boolVal, ok := st.ValueAny.(bool)
		if !ok {
			if st.ValueAny == nil && st.Field.Required {
				return "Campo obrigatório"
			}
			return "Valor inválido"
		}
		st.BoolValue = boolVal
		st.ValuePayload.BoolValue = &boolVal
		st.ValuePayload.HasPayload = true
	case "TEXT":
		raw, _ := st.ValueAny.(string)
		st.Value = raw
		if raw == "" {
			if st.Field.Required {
				return "Campo obrigatório"
			}
			if isEdit && st.HasValue {
				st.ValuePayload.ShouldDelete = true
			}
			return ""
		}
		txt := raw
		st.ValuePayload.TextValue = &txt
		st.ValuePayload.HasPayload = true
	case "INT":
		val, ok := st.ValueAny.(int64)
		if !ok {
			if st.ValueAny == nil {
				if st.Field.Required {
					return "Campo obrigatório"
				}
				if isEdit && st.HasValue {
					st.ValuePayload.ShouldDelete = true
				}
				return ""
			}
			return "Valor inválido"
		}
		st.Value = strconv.FormatInt(val, 10)
		st.ValuePayload.IntValue = &val
		st.ValuePayload.HasPayload = true
	case "FLOAT":
		val, ok := st.ValueAny.(float64)
		if !ok {
			if st.ValueAny == nil {
				if st.Field.Required {
					return "Campo obrigatório"
				}
				if isEdit && st.HasValue {
					st.ValuePayload.ShouldDelete = true
				}
				return ""
			}
			return "Valor inválido"
		}
		st.Value = strconv.FormatFloat(val, 'f', -1, 64)
		st.ValuePayload.FloatValue = &val
		st.ValuePayload.HasPayload = true
	case "DATETIME":
		t, ok := st.ValueAny.(time.Time)
		if !ok {
			if valPtr, okPtr := st.ValueAny.(*time.Time); okPtr && valPtr != nil {
				t = *valPtr
				ok = true
			}
		}
		if !ok {
			if st.ValueAny == nil {
				if st.Field.Required {
					return "Campo obrigatório"
				}
				if isEdit && st.HasValue {
					st.ValuePayload.ShouldDelete = true
				}
				return ""
			}
			return "Valor inválido"
		}
		st.Value = formatDatetimeForInput(t)
		tCopy := t
		st.ValuePayload.TimeValue = &tCopy
		st.ValuePayload.HasPayload = true
	}

	return ""
}

func hydrateFieldStates(fields []db.EAVField, valueMap map[int64]db.EAVValue) []runtimeFieldState {
	states := make([]runtimeFieldState, 0, len(fields))
	for _, fld := range fields {
		state := runtimeFieldState{
			Field:       fld,
			InputName:   runtimeFieldInputName(fld.ID),
			ColumnWidth: normalizeColumnWidth(fld.ColumnWidth),
			IsUIOnly:    fld.IsUI,
			ReadOnly:    fld.IsReadonly,
			ValueAny:    nil,
		}
		if val, ok := valueMap[fld.ID]; ok {
			state.applyExistingValue(val)
		}
		if state.Field.PrimitiveKind == "BOOL" && state.ValueAny == nil {
			state.ValueAny = state.BoolValue
		}
		if fld.IsUI {
			states = append(states, state)
			continue
		}
		states = append(states, state)
	}
	return states
}

func processFieldSubmission(r *http.Request, fields []db.EAVField, valueMap map[int64]db.EAVValue, isEdit bool) ([]runtimeFieldState, bool) {
	states := make([]runtimeFieldState, 0, len(fields))
	hasErr := false

	for _, fld := range fields {
		state := runtimeFieldState{
			Field:       fld,
			InputName:   runtimeFieldInputName(fld.ID),
			ColumnWidth: normalizeColumnWidth(fld.ColumnWidth),
			IsUIOnly:    fld.IsUI,
			ReadOnly:    fld.IsReadonly,
			ValueAny:    nil,
		}
		if val, ok := valueMap[fld.ID]; ok {
			state.applyExistingValue(val)
		}

		impl, ok := ui.Get(fld.UIKind)
		if !ok {
			state.Error = "Tipo de UI inválido"
			hasErr = true
			states = append(states, state)
			continue
		}

		if fld.PrimitiveKind == "BOOL" && state.ValueAny == nil {
			state.ValueAny = state.BoolValue
		}

		if state.ReadOnly {
			raw, submitted := submittedValue(r, state.InputName)
			if submitted {
				raw = strings.TrimSpace(raw)
				if !state.HasValue {
					if raw != "" {
						state.Error = "Campo somente leitura"
						hasErr = true
					}
				} else {
					switch fld.PrimitiveKind {
					case "BOOL":
						attempt := raw == "on" || raw == "1" || strings.EqualFold(raw, "true")
						if attempt != state.BoolValue {
							state.Error = "Campo somente leitura"
							hasErr = true
						}
					default:
						if raw != state.Value {
							state.Error = "Campo somente leitura"
							hasErr = true
						}
					}
				}
			}
			states = append(states, state)
			continue
		}

		if impl.HasPersistence() && !state.IsUIOnly {
			raw := strings.TrimSpace(r.FormValue(state.InputName))
			switch fld.PrimitiveKind {
			case "BOOL":
				checked := raw == "on" || raw == "1" || strings.EqualFold(raw, "true")
				state.BoolValue = checked
				state.ValueAny = checked
				if !checked && fld.Required {
					state.Error = "Campo obrigatório"
					hasErr = true
				}
			case "TEXT":
				state.Value = raw
				if raw == "" {
					if fld.Required {
						state.Error = "Campo obrigatório"
						hasErr = true
					}
				} else {
					state.ValueAny = raw
				}
			case "INT":
				state.Value = raw
				if raw == "" {
					if fld.Required {
						state.Error = "Campo obrigatório"
						hasErr = true
					}
				} else {
					parsed, err := strconv.ParseInt(raw, 10, 64)
					if err != nil {
						state.Error = "Valor inválido"
						hasErr = true
					} else {
						state.ValueAny = parsed
					}
				}
			case "FLOAT":
				state.Value = raw
				if raw == "" {
					if fld.Required {
						state.Error = "Campo obrigatório"
						hasErr = true
					}
				} else {
					parsed, err := strconv.ParseFloat(raw, 64)
					if err != nil {
						state.Error = "Valor inválido"
						hasErr = true
					} else {
						state.ValueAny = parsed
					}
				}
			case "DATETIME":
				state.Value = raw
				if raw == "" {
					if fld.Required {
						state.Error = "Campo obrigatório"
						hasErr = true
					}
				} else {
					parsed, err := parseDatetimeInput(raw)
					if err != nil {
						state.Error = "Data inválida"
						hasErr = true
					} else {
						state.ValueAny = *parsed
					}
				}
			}
		}

		states = append(states, state)
	}

	return states, hasErr
}

func validateAndTransformStates(states []runtimeFieldState, recordID *int64, isEdit bool) ([]runtimeFieldState, bool) {
	allValues := buildAllValues(states)
	hasErr := false

	for idx := range states {
		st := &states[idx]
		impl, ok := ui.Get(st.Field.UIKind)
		if !ok {
			st.Error = "Tipo de UI inválido"
			hasErr = true
			continue
		}

		ctx := ui.FieldRuntimeContext{
			FormID:      st.Field.FormID,
			RecordID:    recordID,
			Field:       &st.Field,
			Value:       st.ValueAny,
			AllValues:   allValues,
			IsNewRecord: !isEdit,
		}

		res := impl.Validate(ctx)
		if !res.OK {
			st.Error = res.Message
			hasErr = true
		}

		if st.Error != "" || st.IsUIOnly || st.ReadOnly {
			continue
		}

		value, err := impl.BeforeSave(&ctx)
		if err != nil {
			st.Error = "Erro ao processar valor"
			hasErr = true
			continue
		}
		st.ValueAny = value
		allValues[st.Field.MachineName] = st.ValueAny

		if errMsg := applyValueToPayload(st, isEdit); errMsg != "" {
			st.Error = errMsg
			hasErr = true
		}
	}

	return states, hasErr
}

func renderNodeEdit(node *runtimeFieldNode, allValues map[string]any, recordID *int64, isNew bool, parentAccordion string) error {
	childRows, err := renderChildrenEdit(node.Children, allValues, recordID, isNew)
	if err != nil {
		return err
	}

	impl, ok := ui.Get(node.State.Field.UIKind)
	if !ok {
		node.State.Error = "Tipo de UI inválido"
		return nil
	}

	ctx := ui.FieldRuntimeContext{
		FormID:      node.State.Field.FormID,
		RecordID:    recordID,
		Field:       &node.State.Field,
		Value:       node.State.ValueAny,
		AllValues:   allValues,
		IsNewRecord: isNew,
	}

	_, opts, err := impl.RenderOptions(ctx)
	if err != nil {
		return err
	}

	data := ui.TemplateData{
		Context:          ctx,
		Options:          opts,
		InputName:        node.State.InputName,
		Error:            node.State.Error,
		ReadOnly:         node.State.ReadOnly,
		Value:            node.State.ValueAny,
		Display:          node.State.Value,
		ChildrenRowsEdit: childRows,
		ParentAccordion:  parentAccordion,
	}

	rendered, err := ui.RenderEditTemplate(impl, data)
	if err != nil {
		return err
	}
	node.State.RenderedEdit = template.HTML(rendered)
	return nil
}

func renderNodeView(node *runtimeFieldNode, allValues map[string]any, recordID *int64, parentAccordion string) error {
	childRows, err := renderChildrenView(node.Children, allValues, recordID)
	if err != nil {
		return err
	}

	impl, ok := ui.Get(node.State.Field.UIKind)
	if !ok {
		node.State.Error = "Tipo de UI inválido"
		return nil
	}

	ctx := ui.FieldRuntimeContext{
		FormID:      node.State.Field.FormID,
		RecordID:    recordID,
		Field:       &node.State.Field,
		Value:       node.State.ValueAny,
		AllValues:   allValues,
		IsNewRecord: false,
	}

	_, opts, err := impl.RenderOptions(ctx)
	if err != nil {
		return err
	}

	data := ui.TemplateData{
		Context:          ctx,
		Options:          opts,
		Value:            node.State.ValueAny,
		Display:          node.State.Value,
		ChildrenRowsView: childRows,
		ParentAccordion:  parentAccordion,
	}

	rendered, err := ui.RenderViewTemplate(impl, data)
	if err != nil {
		return err
	}
	node.State.RenderedView = template.HTML(rendered)
	return nil
}

func renderChildrenEdit(children []*runtimeFieldNode, allValues map[string]any, recordID *int64, isNew bool) ([]ui.ChildRow, error) {
	rows := make([]ui.ChildRow, 0)
	current := ui.ChildRow{}
	currentWidth := 0

	idx := 0
	for idx < len(children) {
		node := children[idx]
		width := normalizeColumnWidth(node.State.ColumnWidth)
		node.State.ColumnWidth = width

		var content template.HTML
		if node.State.Field.UIKind == "group_accordion" {
			key := accordionGroupKey(node.State.Field.UIMetaJSON)
			parentID := accordionContainerID(key, node.State.Field.ID)
			items := make([]template.HTML, 0)
			for idx < len(children) {
				candidate := children[idx]
				if candidate.State.Field.UIKind != "group_accordion" {
					break
				}
				candidateKey := accordionGroupKey(candidate.State.Field.UIMetaJSON)
				if candidateKey != key {
					break
				}
				err := renderNodeEdit(candidate, allValues, recordID, isNew, parentID)
				if err != nil {
					return nil, err
				}
				items = append(items, candidate.State.RenderedEdit)
				idx++
			}
			content = template.HTML(fmt.Sprintf("<div class=\"accordion mb-3\" id=\"%s\">%s</div>", parentID, strings.Join(htmlSliceToString(items), "")))
		} else {
			err := renderNodeEdit(node, allValues, recordID, isNew, "")
			if err != nil {
				return nil, err
			}
			content = node.State.RenderedEdit
			idx++
		}

		if currentWidth+width > 12 && len(current.Children) > 0 {
			rows = append(rows, current)
			current = ui.ChildRow{}
			currentWidth = 0
		}

		current.Children = append(current.Children, ui.ChildItem{ColumnWidth: width, Content: content})
		currentWidth += width
	}

	if len(current.Children) > 0 {
		rows = append(rows, current)
	}

	return rows, nil
}

func renderChildrenView(children []*runtimeFieldNode, allValues map[string]any, recordID *int64) ([]ui.ChildRow, error) {
	rows := make([]ui.ChildRow, 0)
	current := ui.ChildRow{}
	currentWidth := 0

	idx := 0
	for idx < len(children) {
		node := children[idx]
		width := normalizeColumnWidth(node.State.ColumnWidth)
		node.State.ColumnWidth = width

		var content template.HTML
		if node.State.Field.UIKind == "group_accordion" {
			key := accordionGroupKey(node.State.Field.UIMetaJSON)
			parentID := accordionContainerID(key, node.State.Field.ID)
			items := make([]template.HTML, 0)
			for idx < len(children) {
				candidate := children[idx]
				if candidate.State.Field.UIKind != "group_accordion" {
					break
				}
				candidateKey := accordionGroupKey(candidate.State.Field.UIMetaJSON)
				if candidateKey != key {
					break
				}
				err := renderNodeView(candidate, allValues, recordID, parentID)
				if err != nil {
					return nil, err
				}
				items = append(items, candidate.State.RenderedView)
				idx++
			}
			content = template.HTML(fmt.Sprintf("<div class=\"accordion mb-3\" id=\"%s\">%s</div>", parentID, strings.Join(htmlSliceToString(items), "")))
		} else {
			err := renderNodeView(node, allValues, recordID, "")
			if err != nil {
				return nil, err
			}
			content = node.State.RenderedView
			idx++
		}

		if currentWidth+width > 12 && len(current.Children) > 0 {
			rows = append(rows, current)
			current = ui.ChildRow{}
			currentWidth = 0
		}

		current.Children = append(current.Children, ui.ChildItem{ColumnWidth: width, Content: content})
		currentWidth += width
	}

	if len(current.Children) > 0 {
		rows = append(rows, current)
	}

	return rows, nil
}

func htmlSliceToString(items []template.HTML) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = string(item)
	}
	return out
}

func renderEditTree(states []runtimeFieldState, recordID *int64, isNew bool) ([]ui.ChildRow, error) {
	roots := buildFieldTree(states)
	allValues := buildAllValues(states)
	return renderChildrenEdit(roots, allValues, recordID, isNew)
}

func renderViewTree(states []runtimeFieldState, recordID *int64) ([]ui.ChildRow, error) {
	roots := buildFieldTree(states)
	allValues := buildAllValues(states)
	return renderChildrenView(roots, allValues, recordID)
}

func runtimeRecordListHandler(w http.ResponseWriter, r *http.Request) {
	u, authed := checkAuth(w, r)
	if !authed {
		return
	}

	ctx, err := loadRuntimeContext(r.PathValue("workspaceRef"), r.PathValue("formMachineName"))
	if err != nil {
		log.Printf("runtime context error: %v", err)
		http.Error(w, "formulário não encontrado", http.StatusNotFound)
		return
	}

	if !ensureRuntimePermission(w, ctx.Form, u, eavPermView) {
		return
	}

	limit := parsePositiveInt(r.URL.Query(), "limit", 20, 100)
	offset := parsePositiveInt(r.URL.Query(), "offset", 0, 0)
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "updated_desc" // default: last updated first
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	var records []db.EAVRecord

	if query != "" {
		// Use FTS search when query is provided
		records, err = db.Storage.SearchEAVRecords(ctx.Form.ID, query, sort, limit+1, offset)
	} else {
		// Use regular listing when no search query
		records, err = db.Storage.ListEAVRecords(ctx.Form.ID, sort, limit+1, offset)
	}
	if err != nil {
		log.Printf("list records error: %v", err)
		http.Error(w, "erro ao listar registros", http.StatusInternalServerError)
		return
	}

	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}

	nextOffset := offset
	if hasMore {
		nextOffset = offset + limit
	}

	message := strings.TrimSpace(r.URL.Query().Get("message"))
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed     bool
		User       db.User
		Config     config.Config
		Workspace  db.EAVWorkspace
		Form       db.EAVForm
		Records    []db.EAVRecord
		Limit      int
		Offset     int
		NextOffset int
		HasMore    bool
		Message    string
		Sort       string
		Query      string
	}{
		Authed:     true,
		User:       *u,
		Config:     *config.Cfg,
		Workspace:  *ctx.Workspace,
		Form:       *ctx.Form,
		Records:    records,
		Limit:      limit,
		Offset:     offset,
		NextOffset: nextOffset,
		HasMore:    hasMore,
		Message:    message,
		Sort:       sort,
		Query:      query,
	}

	if err := templates.ExecuteTemplate(w, "eav_runtime_record_list.go.tmpl", data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "erro ao renderizar", http.StatusInternalServerError)
	}
}

func runtimeRecordsPartialHandler(w http.ResponseWriter, r *http.Request) {
	u, authed := checkAuth(w, r)
	if !authed {
		return
	}

	ctx, err := loadRuntimeContext(r.PathValue("workspaceRef"), r.PathValue("formMachineName"))
	if err != nil {
		log.Printf("runtime context error: %v", err)
		http.Error(w, "formulário não encontrado", http.StatusNotFound)
		return
	}

	if !ensureRuntimePermission(w, ctx.Form, u, eavPermView) {
		return
	}

	// Get view mode from query parameter
	viewMode := normalizeViewMode(r.URL.Query().Get("view"))

	limit := parsePositiveInt(r.URL.Query(), "limit", 20, 100)
	offset := parsePositiveInt(r.URL.Query(), "offset", 0, 0)
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "updated_desc" // default: last updated first
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// Load records (using search if query provided)
	var records []db.EAVRecord
	if query != "" {
		records, err = db.Storage.SearchEAVRecords(ctx.Form.ID, query, sort, limit+1, offset)
	} else {
		records, err = db.Storage.ListEAVRecords(ctx.Form.ID, sort, limit+1, offset)
	}
	if err != nil {
		log.Printf("list records error: %v", err)
		http.Error(w, "erro ao listar registros", http.StatusInternalServerError)
		return
	}

	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}

	// Load all values for all records and render field rows
	recordsWithValues := make([]recordWithValues, 0, len(records))
	for _, rec := range records {
		values, err := db.Storage.GetEAVValues(rec.ID)
		if err != nil {
			log.Printf("get values error for record %d: %v", rec.ID, err)
			continue
		}
		valueMap := valuesByField(values)

		// Build field rows with pre-rendered HTML for grouping fields
		states := hydrateFieldStates(ctx.Fields, valueMap)
		recordID := rec.ID
		fieldRows, err := renderViewTree(states, &recordID)
		if err != nil {
			log.Printf("render view tree error for record %d: %v", rec.ID, err)
			fieldRows = nil
		}

		// Build display values map for list view
		// The hydrateFieldStates function already formats values into state.Value
		displayValues := make(map[int64]string)
		for _, state := range states {
			displayValues[state.Field.ID] = state.Value
		}

		recordsWithValues = append(recordsWithValues, recordWithValues{
			Record:        rec,
			Values:        valueMap,
			FieldRows:     fieldRows,
			DisplayValues: displayValues,
		})
	}

	// Filter and sort fields based on view mode
	visibleFields := filterFieldsByViewMode(ctx.Fields, viewMode)
	sortFieldsByViewMode(visibleFields, viewMode)

	// Build base path for URLs
	basePath := fmt.Sprintf("/eav/workspaces/%s/forms/%s", ctx.Workspace.ReferenceID, ctx.Form.MachineName)

	data := struct {
		Config        config.Config
		Records       []recordWithValues
		VisibleFields []db.EAVField
		ViewMode      string
		BasePath      string
		Workspace     db.EAVWorkspace
		Form          db.EAVForm
		Sort          string
		Query         string
	}{
		Config:        *config.Cfg,
		Records:       recordsWithValues,
		VisibleFields: visibleFields,
		Sort:          sort,
		Query:         query,
		ViewMode:      viewMode,
		BasePath:      basePath,
		Workspace:     *ctx.Workspace,
		Form:          *ctx.Form,
	}

	// Render appropriate partial template based on view mode
	var templateName string
	switch viewMode {
	case "list":
		templateName = "eav_records_list"
	case "card":
		templateName = "eav_records_card"
	case "carousel":
		templateName = "eav_records_carousel"
	default:
		templateName = "eav_records_list"
	}

	if err := templates.ExecuteTemplate(w, templateName, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "erro ao renderizar", http.StatusInternalServerError)
	}
}

type recordWithValues struct {
	Record        db.EAVRecord
	Values        map[int64]db.EAVValue
	FieldRows     []ui.ChildRow
	DisplayValues map[int64]string
}

func runtimeRecordCreateHandler(w http.ResponseWriter, r *http.Request) {
	u, authed := checkAuth(w, r)
	if !authed {
		return
	}

	ctx, err := loadRuntimeContext(r.PathValue("workspaceRef"), r.PathValue("formMachineName"))
	if err != nil {
		http.Error(w, "formulário não encontrado", http.StatusNotFound)
		return
	}

	if !ctx.Form.Active {
		http.Error(w, "formulário inativo", http.StatusForbidden)
		return
	}

	if !ensureRuntimePermission(w, ctx.Form, u, eavPermCreate) {
		return
	}

	renderForm := func(states []runtimeFieldState, statusCode int, formErrors []string) {
		rows, renderErr := renderEditTree(states, nil, true)
		if renderErr != nil {
			log.Printf("render field error: %v", renderErr)
			http.Error(w, "erro ao renderizar", http.StatusInternalServerError)
			return
		}

		if statusCode != 0 {
			w.WriteHeader(statusCode)
		}
		data := struct {
			Authed    bool
			User      db.User
			Config    config.Config
			Workspace db.EAVWorkspace
			Form      db.EAVForm
			Fields    []runtimeFieldState
			FieldRows []ui.ChildRow
			Csrf      string
			ActionURL string
			BackURL   string
			Mode      string
			Errors    []string
			Record    *db.EAVRecord
			DeleteURL string
		}{
			Authed:    true,
			User:      *u,
			Config:    *config.Cfg,
			Workspace: *ctx.Workspace,
			Form:      *ctx.Form,
			Fields:    states,
			FieldRows: rows,
			Csrf:      session.GenerateCSRFToken(w, r),
			ActionURL: r.URL.Path,
			BackURL:   fmt.Sprintf("/eav/workspaces/%s/forms/%s/records", ctx.Workspace.ReferenceID, ctx.Form.MachineName),
			Mode:      "create",
			Errors:    formErrors,
			DeleteURL: "",
		}
		if err := templates.ExecuteTemplate(w, "eav_runtime_record_form.go.tmpl", data); err != nil {
			log.Printf("template error: %v", err)
			http.Error(w, "erro ao renderizar", http.StatusInternalServerError)
		}
	}

	fields := ctx.renderableFields()
	if r.Method != http.MethodPost {
		renderForm(hydrateFieldStates(fields, nil), 0, nil)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Printf("parse form error: %v", err)
		http.Error(w, "dados inválidos", http.StatusBadRequest)
		return
	}

	if !session.ValidateCSRF(r) {
		http.Error(w, "CSRF inválido", http.StatusForbidden)
		return
	}

	states, hasErr := processFieldSubmission(r, fields, nil, false)
	states, valErr := validateAndTransformStates(states, nil, false)
	if valErr {
		hasErr = true
	}
	if hasErr {
		renderForm(states, http.StatusBadRequest, []string{"Verifique os campos destacados."})
		return
	}

	record, err := db.Storage.CreateEAVRecord(ctx.Form.ID, ctx.Workspace.ID, u.ID, "active", "{}")
	if err != nil {
		log.Printf("create record error: %v", err)
		http.Error(w, "erro ao criar registro", http.StatusInternalServerError)
		return
	}

	if err := persistFieldStates(record.ID, ctx.Form.ID, states); err != nil {
		log.Printf("persist values error: %v", err)
		http.Error(w, "erro ao salvar valores", http.StatusInternalServerError)
		return
	}

	redirectURL := fmt.Sprintf("/eav/workspaces/%s/forms/%s/records?message=%s",
		ctx.Workspace.ReferenceID,
		ctx.Form.MachineName,
		url.QueryEscape("Registro criado com sucesso."),
	)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func persistFieldStates(recordID, formID int64, states []runtimeFieldState) error {
	for _, st := range states {
		if st.IsUIOnly || st.ReadOnly {
			continue
		}
		payload := st.ValuePayload
		if payload.ShouldDelete {
			if err := db.Storage.DeleteEAVValue(recordID, st.Field.ID); err != nil {
				return err
			}
			continue
		}
		if !payload.HasPayload {
			continue
		}
		if err := db.Storage.SetEAVValue(recordID, st.Field.ID, formID, payload.BoolValue, payload.TimeValue, payload.FloatValue, payload.IntValue, payload.TextValue); err != nil {
			return err
		}
	}
	return nil
}

func runtimeRecordViewHandler(w http.ResponseWriter, r *http.Request) {
	u, authed := checkAuth(w, r)
	if !authed {
		return
	}

	ctx, err := loadRuntimeContext(r.PathValue("workspaceRef"), r.PathValue("formMachineName"))
	if err != nil {
		http.Error(w, "formulário não encontrado", http.StatusNotFound)
		return
	}

	if !ensureRuntimePermission(w, ctx.Form, u, eavPermView) {
		return
	}

	record, err := db.Storage.GetEAVRecordByReference(r.PathValue("recordRef"))
	if err != nil || record.FormID != ctx.Form.ID {
		http.Error(w, "registro não encontrado", http.StatusNotFound)
		return
	}

	values, err := db.Storage.GetEAVValues(record.ID)
	if err != nil {
		log.Printf("get values error: %v", err)
		http.Error(w, "erro ao carregar registro", http.StatusInternalServerError)
		return
	}

	valueMap := valuesByField(values)
	fields := ctx.renderableFields()
	states := hydrateFieldStates(fields, valueMap)
	rows, err := renderViewTree(states, &record.ID)
	if err != nil {
		log.Printf("render view error: %v", err)
		http.Error(w, "erro ao renderizar", http.StatusInternalServerError)
		return
	}

	message := strings.TrimSpace(r.URL.Query().Get("message"))
	if len(message) > 200 {
		message = ""
	}

	data := struct {
		Authed    bool
		User      db.User
		Config    config.Config
		Workspace db.EAVWorkspace
		Form      db.EAVForm
		Record    db.EAVRecord
		Fields    []runtimeFieldState
		FieldRows []ui.ChildRow
		Message   string
		Csrf      string
	}{
		Authed:    true,
		User:      *u,
		Config:    *config.Cfg,
		Workspace: *ctx.Workspace,
		Form:      *ctx.Form,
		Record:    *record,
		Fields:    states,
		FieldRows: rows,
		Message:   message,
		Csrf:      session.GenerateCSRFToken(w, r),
	}

	if err := templates.ExecuteTemplate(w, "eav_runtime_record_view.go.tmpl", data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "erro ao renderizar", http.StatusInternalServerError)
	}
}

func runtimeRecordEditHandler(w http.ResponseWriter, r *http.Request) {
	u, authed := checkAuth(w, r)
	if !authed {
		return
	}

	ctx, err := loadRuntimeContext(r.PathValue("workspaceRef"), r.PathValue("formMachineName"))
	if err != nil {
		http.Error(w, "formulário não encontrado", http.StatusNotFound)
		return
	}

	if !ensureRuntimePermission(w, ctx.Form, u, eavPermEdit) {
		return
	}

	record, err := db.Storage.GetEAVRecordByReference(r.PathValue("recordRef"))
	if err != nil || record.FormID != ctx.Form.ID {
		http.Error(w, "registro não encontrado", http.StatusNotFound)
		return
	}

	values, err := db.Storage.GetEAVValues(record.ID)
	if err != nil {
		log.Printf("get values error: %v", err)
		http.Error(w, "erro ao carregar registro", http.StatusInternalServerError)
		return
	}
	valueMap := valuesByField(values)
	fields := ctx.renderableFields()

	renderForm := func(states []runtimeFieldState, statusCode int, formErrors []string) {
		rows, renderErr := renderEditTree(states, &record.ID, false)
		if renderErr != nil {
			log.Printf("render field error: %v", renderErr)
			http.Error(w, "erro ao renderizar", http.StatusInternalServerError)
			return
		}

		if statusCode != 0 {
			w.WriteHeader(statusCode)
		}
		data := struct {
			Authed    bool
			User      db.User
			Config    config.Config
			Workspace db.EAVWorkspace
			Form      db.EAVForm
			Fields    []runtimeFieldState
			FieldRows []ui.ChildRow
			Csrf      string
			ActionURL string
			BackURL   string
			Mode      string
			Errors    []string
			Record    *db.EAVRecord
			DeleteURL string
		}{
			Authed:    true,
			User:      *u,
			Config:    *config.Cfg,
			Workspace: *ctx.Workspace,
			Form:      *ctx.Form,
			Fields:    states,
			FieldRows: rows,
			Csrf:      session.GenerateCSRFToken(w, r),
			ActionURL: r.URL.Path,
			BackURL:   fmt.Sprintf("/eav/workspaces/%s/forms/%s/records/%s", ctx.Workspace.ReferenceID, ctx.Form.MachineName, record.ReferenceID),
			Mode:      "edit",
			Errors:    formErrors,
			Record:    record,
			DeleteURL: fmt.Sprintf("/eav/workspaces/%s/forms/%s/records/%s/delete", ctx.Workspace.ReferenceID, ctx.Form.MachineName, record.ReferenceID),
		}
		if err := templates.ExecuteTemplate(w, "eav_runtime_record_form.go.tmpl", data); err != nil {
			log.Printf("template error: %v", err)
			http.Error(w, "erro ao renderizar", http.StatusInternalServerError)
		}
	}

	if r.Method != http.MethodPost {
		renderForm(hydrateFieldStates(fields, valueMap), 0, nil)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Printf("parse form error: %v", err)
		http.Error(w, "dados inválidos", http.StatusBadRequest)
		return
	}

	if !session.ValidateCSRF(r) {
		http.Error(w, "CSRF inválido", http.StatusForbidden)
		return
	}

	states, hasErr := processFieldSubmission(r, fields, valueMap, true)
	states, valErr := validateAndTransformStates(states, &record.ID, true)
	if valErr {
		hasErr = true
	}
	if hasErr {
		renderForm(states, http.StatusBadRequest, []string{"Verifique os campos destacados."})
		return
	}

	rev, err := strconv.Atoi(r.FormValue("record_rev"))
	if err != nil {
		renderForm(states, http.StatusBadRequest, []string{"Versão do registro inválida."})
		return
	}
	updated, err := db.Storage.UpdateEAVRecord(record.ID, rev, record.Status, record.TagsJSON)
	if err != nil {
		if strings.Contains(err.Error(), "optimistic") {
			renderForm(states, http.StatusConflict, []string{"O registro foi alterado por outra pessoa. Atualize a página."})
			return
		}
		log.Printf("update record error: %v", err)
		http.Error(w, "erro ao atualizar registro", http.StatusInternalServerError)
		return
	}
	record = updated

	if err := persistFieldStates(record.ID, ctx.Form.ID, states); err != nil {
		log.Printf("persist values error: %v", err)
		http.Error(w, "erro ao salvar valores", http.StatusInternalServerError)
		return
	}

	redirectURL := fmt.Sprintf("/eav/workspaces/%s/forms/%s/records/%s?message=%s",
		ctx.Workspace.ReferenceID,
		ctx.Form.MachineName,
		record.ReferenceID,
		url.QueryEscape("Registro atualizado."),
	)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func runtimeRecordDeleteHandler(w http.ResponseWriter, r *http.Request) {
	u, authed := checkAuth(w, r)
	if !authed {
		return
	}

	ctx, err := loadRuntimeContext(r.PathValue("workspaceRef"), r.PathValue("formMachineName"))
	if err != nil {
		http.Error(w, "formulário não encontrado", http.StatusNotFound)
		return
	}

	if !ensureRuntimePermission(w, ctx.Form, u, eavPermDelete) {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "dados inválidos", http.StatusBadRequest)
		return
	}

	if !session.ValidateCSRF(r) {
		http.Error(w, "CSRF inválido", http.StatusForbidden)
		return
	}

	if r.FormValue("confirm_delete") != "on" {
		http.Error(w, "confirmação obrigatória", http.StatusBadRequest)
		return
	}

	record, err := db.Storage.GetEAVRecordByReference(r.PathValue("recordRef"))
	if err != nil || record.FormID != ctx.Form.ID {
		http.Error(w, "registro não encontrado", http.StatusNotFound)
		return
	}

	if err := db.Storage.SoftDeleteEAVRecord(record.ID); err != nil {
		log.Printf("delete record error: %v", err)
		http.Error(w, "erro ao excluir registro", http.StatusInternalServerError)
		return
	}

	redirectURL := fmt.Sprintf("/eav/workspaces/%s/forms/%s/records?message=%s",
		ctx.Workspace.ReferenceID,
		ctx.Form.MachineName,
		url.QueryEscape("Registro excluído com sucesso."),
	)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
