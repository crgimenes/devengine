package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/ratelimit"
)

// REST projection of a form (8.1): a form flagged expose_api answers under
// /api/v1/{machine_name} with the entity rules (required, unique, computed,
// pre_save) and the form rules (element subset, validate_expr) inherited
// from the same pipeline the HTML runtime uses. Authentication is a bearer
// token minted on /me; only the SHA-256 of the token is stored.

const apiListLimit = 50

// apiJSON writes v as the JSON response body.
func apiJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// apiError writes a JSON error envelope. fields carries per-field messages
// (validation) and is omitted when empty.
func apiError(w http.ResponseWriter, status int, msg string, fields map[string]string) {
	body := map[string]any{"error": msg}
	if len(fields) > 0 {
		body["fields"] = fields
	}
	apiJSON(w, status, body)
}

// hashAPIToken is the storage form of a bearer token.
func hashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// apiUser authenticates the request via Authorization: Bearer. Failed
// attempts consume the per-IP auth budget so tokens cannot be brute forced.
func (h *Handlers) apiUser(w http.ResponseWriter, r *http.Request) (*db.User, bool) {
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || token == "" {
		apiError(w, http.StatusUnauthorized, "missing bearer token", nil)
		return nil, false
	}

	user, err := db.Storage.GetUserByAPITokenHash(hashAPIToken(token))
	if err != nil {
		ref := logRef("apiUser", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return nil, false
	}
	if user == nil {
		ip := ratelimit.ClientIP(r, h.cfg.RateLimitTrustProxy)
		if !ratelimit.Default.Allow(ip, h.cfg.RateLimitPerMin, h.cfg.RateLimitBurst) {
			w.Header().Set("Retry-After", "60")
			apiError(w, http.StatusTooManyRequests, "too many attempts", nil)
			return nil, false
		}
		apiError(w, http.StatusUnauthorized, "invalid token", nil)
		return nil, false
	}
	return user, true
}

// loadAPIForm resolves {machineName} to an API-exposed, EAV-bound form and
// loads its entity, elements and attributes. Forms not flagged expose_api
// are 404 — their existence is not revealed.
func (h *Handlers) loadAPIForm(w http.ResponseWriter, r *http.Request) (runtimeFormContext, bool) {
	var ctx runtimeFormContext

	form, err := db.Storage.GetFormByMachineName(r.PathValue("machineName"))
	if err != nil {
		ref := logRef("loadAPIForm", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return ctx, false
	}
	if form == nil || !form.ExposeAPI || form.EAVEntityTypeID == nil {
		apiError(w, http.StatusNotFound, "not found", nil)
		return ctx, false
	}
	ctx.form = form

	ctx.entityType, err = db.Storage.GetEAVEntityTypeByID(*form.EAVEntityTypeID)
	if err != nil {
		ref := logRef("loadAPIForm entity", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return ctx, false
	}
	ctx.attributes, err = db.Storage.ListEAVAttributesByEntityTypeID(ctx.entityType.ID)
	if err != nil {
		ref := logRef("loadAPIForm attributes", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return ctx, false
	}
	ctx.elements, err = db.Storage.ListFormElements(form.ID)
	if err != nil {
		ref := logRef("loadAPIForm elements", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return ctx, false
	}
	ctx.attrMap = make(map[int64]*db.EAVAttribute, len(ctx.attributes))
	for i := range ctx.attributes {
		ctx.attrMap[ctx.attributes[i].ID] = &ctx.attributes[i]
	}
	return ctx, true
}

// exposedAttributes returns the attributes bound by the form's elements —
// the field subset the API exposes, in element order.
func exposedAttributes(ctx runtimeFormContext) []*db.EAVAttribute {
	var out []*db.EAVAttribute
	seen := map[int64]bool{}
	for _, el := range ctx.elements {
		if el.EAVAttributeID == nil || seen[*el.EAVAttributeID] {
			continue
		}
		attr := ctx.attrMap[*el.EAVAttributeID]
		if attr == nil {
			continue
		}
		seen[attr.ID] = true
		out = append(out, attr)
	}
	return out
}

// apiRecordBody projects a record and its values into the response shape.
func apiRecordBody(ctx runtimeFormContext, rec *db.EAVRecord, vals []db.EAVValue) map[string]any {
	byAttr := make(map[int64]*db.EAVValue, len(vals))
	for i := range vals {
		byAttr[vals[i].AttributeID] = &vals[i]
	}
	values := map[string]any{}
	for _, attr := range exposedAttributes(ctx) {
		v := byAttr[attr.ID]
		if v == nil {
			values[attr.MachineName] = nil
			continue
		}
		switch {
		case v.VBool != nil:
			values[attr.MachineName] = *v.VBool
		case v.VInt != nil:
			values[attr.MachineName] = *v.VInt
		case v.VReal != nil:
			values[attr.MachineName] = *v.VReal
		case v.VText != nil:
			values[attr.MachineName] = *v.VText
		case v.VDatetime != nil:
			values[attr.MachineName] = *v.VDatetime
		default:
			values[attr.MachineName] = nil
		}
	}
	return map[string]any{
		"reference_id": rec.ReferenceID,
		"status":       rec.Status,
		"rev":          rec.Rev,
		"created_at":   rec.CreatedAt,
		"updated_at":   rec.UpdatedAt,
		"values":       values,
	}
}

// parseAPIAttributes mirrors parseFormAttributesLenient for a JSON body:
// values keyed by element machine_name go through the same plugin Parse the
// HTML form uses, so both entry points enforce identical typing rules.
// Absent keys behave like empty form fields (full-document semantics).
func parseAPIAttributes(body map[string]any, elements []db.FormElement, attributes []db.EAVAttribute) (db.EAVRecordValues, map[string]string) {
	values := make(db.EAVRecordValues)
	fieldErrors := make(map[string]string)

	attrByID := make(map[int64]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		attrByID[attributes[i].ID] = &attributes[i]
	}

	for _, el := range elements {
		if el.EAVAttributeID == nil {
			continue
		}
		attr := attrByID[*el.EAVAttributeID]
		if attr == nil {
			continue
		}

		raw, err := jsonValueToRaw(body[el.MachineName])
		if err != nil {
			fieldErrors[el.MachineName] = "invalid value"
			continue
		}
		if raw == "" && attr.IsRequired {
			fieldErrors[el.MachineName] = "required field"
			values[attr.MachineName] = ""
			continue
		}

		// Empty raw still goes through the plugin: Parse("") yields the
		// zero of the primitive kind, exactly like an empty HTML input.
		// Short-circuiting to "" here used to leave a string in INT/REAL
		// fields and numeric validate_exprs aborted on the type mismatch.
		v, err := parseElementValue(el, attr, raw)
		if err != nil {
			fieldErrors[el.MachineName] = "invalid value"
			values[attr.MachineName] = raw
			continue
		}
		values[attr.MachineName] = v
	}
	return values, fieldErrors
}

// jsonValueToRaw converts a decoded JSON value into the raw string form the
// field plugins parse, keeping the API on the exact same pipeline as the
// HTML form.
func jsonValueToRaw(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10), nil
		}
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported JSON value type %T", v)
	}
}

// APIRecordsList handles GET /api/v1/{machineName}: cursor-paginated
// listing with the same ?q= text filter (including one level of reference
// indirection) the HTML listing uses.
func (h *Handlers) APIRecordsList(w http.ResponseWriter, r *http.Request) {
	_, ok := h.apiUser(w, r)
	if !ok {
		return
	}
	ctx, ok := h.loadAPIForm(w, r)
	if !ok {
		return
	}

	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > apiListLimit {
		limit = apiListLimit
	}

	records, err := db.Storage.ListEAVRecordsCursor(ctx.entityType.ID, cursor, limit, r.URL.Query().Get("q"))
	if err != nil {
		ref := logRef("APIRecordsList", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return
	}

	ids := make([]int64, len(records))
	for i, rec := range records {
		ids[i] = rec.ID
	}
	valuesByRecord, err := db.Storage.GetEAVValuesForRecordIDs(ids)
	if err != nil {
		ref := logRef("APIRecordsList values", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return
	}

	out := make([]map[string]any, len(records))
	for i := range records {
		out[i] = apiRecordBody(ctx, &records[i], valuesByRecord[records[i].ID])
	}
	body := map[string]any{"records": out}
	if len(records) == limit {
		body["next_cursor"] = records[len(records)-1].ID
	}
	apiJSON(w, http.StatusOK, body)
}

// apiRecord resolves {recordRef} inside the form's entity. Records of other
// entities are 404 even when the ref exists.
func (h *Handlers) apiRecord(w http.ResponseWriter, r *http.Request, ctx runtimeFormContext) (*db.EAVRecord, bool) {
	rec, err := db.Storage.GetEAVRecordByRefID(r.PathValue("recordRef"))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			apiError(w, http.StatusNotFound, "not found", nil)
			return nil, false
		}
		ref := logRef("apiRecord", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return nil, false
	}
	if rec.EntityTypeID != ctx.entityType.ID {
		apiError(w, http.StatusNotFound, "not found", nil)
		return nil, false
	}
	return rec, true
}

// APIRecordsGet handles GET /api/v1/{machineName}/{recordRef}.
func (h *Handlers) APIRecordsGet(w http.ResponseWriter, r *http.Request) {
	_, ok := h.apiUser(w, r)
	if !ok {
		return
	}
	ctx, ok := h.loadAPIForm(w, r)
	if !ok {
		return
	}
	rec, ok := h.apiRecord(w, r, ctx)
	if !ok {
		return
	}
	vals, err := db.Storage.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		ref := logRef("APIRecordsGet", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return
	}
	apiJSON(w, http.StatusOK, apiRecordBody(ctx, rec, vals))
}

// decodeAPIBody parses the JSON request body into a flat object.
func decodeAPIBody(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	var body map[string]any
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&body)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON body", nil)
		return nil, false
	}
	return body, true
}

// attrDefault returns the attribute's authored default value, typed, and
// whether one is set.
func attrDefault(attr *db.EAVAttribute) (any, bool) {
	switch {
	case attr.DefaultVBool != nil:
		return *attr.DefaultVBool, true
	case attr.DefaultVInt != nil:
		return *attr.DefaultVInt, true
	case attr.DefaultVReal != nil:
		return *attr.DefaultVReal, true
	case attr.DefaultVText != nil:
		return *attr.DefaultVText, true
	case attr.DefaultVDatetime != nil:
		return *attr.DefaultVDatetime, true
	}
	return nil, false
}

// runAPIPipeline applies the shared computed/validate steps and reports
// field errors as a 422. applyDefaults makes attributes absent from the
// body take their authored default — what the HTML form pre-fills on a new
// record. Updates keep it off: PUT replaces the resource with the payload.
func (h *Handlers) runAPIPipeline(w http.ResponseWriter, r *http.Request, user *db.User, ctx runtimeFormContext, body map[string]any, applyDefaults bool) (db.EAVRecordValues, bool) {
	values, fieldErrors := parseAPIAttributes(body, ctx.elements, ctx.attributes)
	if len(fieldErrors) > 0 {
		apiError(w, http.StatusUnprocessableEntity, "validation failed", fieldErrors)
		return nil, false
	}

	if applyDefaults {
		for i := range ctx.attributes {
			attr := &ctx.attributes[i]
			if _, inBody := body[attr.MachineName]; inBody {
				continue
			}
			if d, has := attrDefault(attr); has {
				values[attr.MachineName] = d
			}
		}
	}

	values, err := applyComputedExprs(r.Context(), user, ctx.attributes, values)
	if err != nil {
		ref := logRef("runAPIPipeline computed", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return nil, false
	}

	fieldErrors, sysErr := evaluateValidateExprs(r.Context(), user, ctx.elements, ctx.attributes, values)
	if sysErr != nil {
		ref := logRef("runAPIPipeline validate", sysErr)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return nil, false
	}
	if len(fieldErrors) > 0 {
		apiError(w, http.StatusUnprocessableEntity, "validation failed", fieldErrors)
		return nil, false
	}
	return values, true
}

// apiSaveError maps save failures: authored pre_save messages pass verbatim
// as 422, unique violations as 409, the rest hides behind a log ref.
func apiSaveError(w http.ResponseWriter, scope string, err error) {
	var userErr db.UserError
	if errors.As(err, &userErr) {
		apiError(w, http.StatusUnprocessableEntity, userErr.Error(), nil)
		return
	}
	if errors.Is(err, db.ErrConflict) {
		apiError(w, http.StatusConflict, "conflict", nil)
		return
	}
	ref := logRef(scope, err)
	apiError(w, http.StatusInternalServerError, "could not save (ref "+ref+")", nil)
}

// APIRecordsCreate handles POST /api/v1/{machineName}.
func (h *Handlers) APIRecordsCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := h.apiUser(w, r)
	if !ok {
		return
	}
	ctx, ok := h.loadAPIForm(w, r)
	if !ok {
		return
	}
	body, ok := decodeAPIBody(w, r)
	if !ok {
		return
	}
	values, ok := h.runAPIPipeline(w, r, user, ctx, body, true)
	if !ok {
		return
	}

	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		ref := logRef("APIRecordsCreate begin", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	_, recordRef, err := insertRecordTx(tx, ctx.entityType, ctx.attributes, values)
	if err != nil {
		apiSaveError(w, "APIRecordsCreate", err)
		return
	}
	err = tx.Commit()
	if err != nil {
		ref := logRef("APIRecordsCreate commit", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return
	}
	committed = true

	w.Header().Set("Location", "/api/v1/"+ctx.form.MachineName+"/"+recordRef)
	apiJSON(w, http.StatusCreated, map[string]any{"reference_id": recordRef})
}

// APIRecordsUpdate handles PUT /api/v1/{machineName}/{recordRef}. The body
// must carry "rev" matching the current record for optimistic locking.
func (h *Handlers) APIRecordsUpdate(w http.ResponseWriter, r *http.Request) {
	user, ok := h.apiUser(w, r)
	if !ok {
		return
	}
	ctx, ok := h.loadAPIForm(w, r)
	if !ok {
		return
	}
	rec, ok := h.apiRecord(w, r, ctx)
	if !ok {
		return
	}
	body, ok := decodeAPIBody(w, r)
	if !ok {
		return
	}

	rev, isNum := body["rev"].(float64)
	if !isNum {
		apiError(w, http.StatusBadRequest, `"rev" (current record revision) is required`, nil)
		return
	}
	if int(rev) != rec.Rev {
		apiError(w, http.StatusConflict, "stale rev: record changed since it was read", nil)
		return
	}

	values, ok := h.runAPIPipeline(w, r, user, ctx, body, false)
	if !ok {
		return
	}

	tx, err := db.Storage.BeginTransaction()
	if err != nil {
		ref := logRef("APIRecordsUpdate begin", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	err = updateRecordTx(tx, ctx.entityType, rec, ctx.attributes, values)
	if err != nil {
		apiSaveError(w, "APIRecordsUpdate", err)
		return
	}
	err = tx.Commit()
	if err != nil {
		ref := logRef("APIRecordsUpdate commit", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return
	}
	committed = true

	apiJSON(w, http.StatusOK, map[string]any{"reference_id": rec.ReferenceID, "rev": rec.Rev + 1})
}

// APIRecordsDelete handles DELETE /api/v1/{machineName}/{recordRef} as a
// soft delete, matching the HTML tooling.
func (h *Handlers) APIRecordsDelete(w http.ResponseWriter, r *http.Request) {
	_, ok := h.apiUser(w, r)
	if !ok {
		return
	}
	ctx, ok := h.loadAPIForm(w, r)
	if !ok {
		return
	}
	rec, ok := h.apiRecord(w, r, ctx)
	if !ok {
		return
	}
	err := db.Storage.SoftDeleteEAVRecord(rec.ID)
	if err != nil {
		ref := logRef("APIRecordsDelete", err)
		apiError(w, http.StatusInternalServerError, "internal error (ref "+ref+")", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
