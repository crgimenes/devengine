package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/db"
)

// adminEntity bundles what the admin records CRUD tests need.
type adminEntity struct {
	et    *db.EAVEntityType
	attrs map[string]db.EAVAttribute
}

// seedAdminEntity creates an entity type exercising every primitive kind plus
// the validation knobs (required, unique, max_length).
func seedAdminEntity(t *testing.T, s db.Store, preSave string) adminEntity {
	t.Helper()
	et, err := s.CreateEAVEntityType("Item", "item", "", preSave, "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}

	maxLen := 10
	specs := []struct {
		machine, kind string
		required      bool
		unique        bool
		maxLength     *int
	}{
		{"titulo", "TEXT", true, false, &maxLen},
		{"codigo", "TEXT", false, true, nil},
		{"prioridade", "INT", false, false, nil},
		{"peso", "REAL", false, false, nil},
		{"feito", "BOOL", false, false, nil},
		{"prazo", "DATETIME", false, false, nil},
	}

	attrs := make(map[string]db.EAVAttribute, len(specs))
	for _, sp := range specs {
		a, err := s.CreateEAVAttribute(et.ID, sp.machine, sp.machine, "", sp.kind,
			sp.required, sp.unique, false, sp.maxLength, false, "", nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateEAVAttribute(%s): %v", sp.machine, err)
		}
		attrs[sp.machine] = *a
	}
	return adminEntity{et: et, attrs: attrs}
}

func fullRecordForm() url.Values {
	return url.Values{
		"attr_titulo":     {"Comprar"},
		"attr_codigo":     {"SKU-1"},
		"attr_prioridade": {"3"},
		"attr_peso":       {"1.5"},
		"attr_feito":      {"1"},
		"attr_prazo":      {"2026-06-15T18:00"},
	}
}

// recordValues reads back the typed values of the newest record of the entity.
func recordValues(t *testing.T, s db.Store, ent adminEntity) (*db.EAVRecord, map[string]any) {
	t.Helper()
	records, err := s.ListEAVRecordsCursor(ent.et.ID, 0, 10, "")
	if err != nil {
		t.Fatalf("ListEAVRecordsCursor: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("no records found")
	}
	rec := records[0]
	vals, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID: %v", err)
	}
	byAttrID := make(map[int64]string, len(ent.attrs))
	kinds := make(map[string]string, len(ent.attrs))
	for name, a := range ent.attrs {
		byAttrID[a.ID] = name
		kinds[name] = a.PrimitiveKind
	}
	out := make(map[string]any, len(vals))
	for _, v := range vals {
		name := byAttrID[v.AttributeID]
		out[name] = db.UnwrapEAVValue(kinds[name], v)
	}
	return &rec, out
}

func adminRecordsBase(ent adminEntity) string {
	return "/tools/database-schema/eav/" + ent.et.ReferenceID + "/records"
}

// location returns the redirect target with the query unescaped so message
// assertions can match accented words.
func location(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	loc, err := url.QueryUnescape(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("unescape Location: %v", err)
	}
	return loc
}

func TestAdminRecordCreateAllKinds(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, "")
	admin := plantUser(t, "admin", true)

	rr := doPostForm(t, mux, adminRecordsBase(ent)+"/new", fullRecordForm(), admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("create = %d, want 303", rr.Code)
	}
	loc := location(t, rr)
	if !strings.Contains(loc, "successfully") {
		t.Fatalf("create redirected with: %s", loc)
	}

	rec, vals := recordValues(t, s, ent)
	if rec.Status != "active" {
		t.Fatalf("status = %q, want active", rec.Status)
	}
	if vals["titulo"] != "Comprar" {
		t.Fatalf("titulo = %v", vals["titulo"])
	}
	if vals["prioridade"] != int64(3) {
		t.Fatalf("prioridade = %v (%T)", vals["prioridade"], vals["prioridade"])
	}
	if vals["peso"] != 1.5 {
		t.Fatalf("peso = %v", vals["peso"])
	}
	if vals["feito"] != true {
		t.Fatalf("feito = %v", vals["feito"])
	}
	prazo, _ := vals["prazo"].(string)
	if !strings.HasPrefix(prazo, "2026-06-15T18:00") {
		t.Fatalf("prazo = %v", vals["prazo"])
	}
}

// Optional fields left empty must not break the save (the runtime path had
// exactly this bug: an all-NULL eav_values row violates the one-value CHECK).
func TestAdminRecordCreateEmptyOptionals(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, "")
	admin := plantUser(t, "admin", true)

	form := url.Values{"attr_titulo": {"Minimo"}}
	rr := doPostForm(t, mux, adminRecordsBase(ent)+"/new", form, admin)
	loc := location(t, rr)
	if !strings.Contains(loc, "successfully") {
		t.Fatalf("create with empty optionals redirected with: %s", loc)
	}

	_, vals := recordValues(t, s, ent)
	if vals["titulo"] != "Minimo" {
		t.Fatalf("titulo = %v", vals["titulo"])
	}
}

func TestAdminRecordCreateValidation(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, "")
	admin := plantUser(t, "admin", true)

	tests := []struct {
		name    string
		mutate  func(url.Values)
		wantMsg string
	}{
		{"missing required", func(f url.Values) { f.Del("attr_titulo") }, "Required field"},
		{"invalid int", func(f url.Values) { f.Set("attr_prioridade", "abc") }, "Invalid value"},
		{"text over max length", func(f url.Values) { f.Set("attr_titulo", "12345678901") }, "exceeds the limit"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			form := fullRecordForm()
			tc.mutate(form)
			rr := doPostForm(t, mux, adminRecordsBase(ent)+"/new", form, admin)
			loc := location(t, rr)
			if !strings.Contains(loc, tc.wantMsg) {
				t.Fatalf("want %q in redirect, got: %s", tc.wantMsg, loc)
			}
		})
	}
}

func TestAdminRecordCreateUniqueViolation(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, "")
	admin := plantUser(t, "admin", true)

	rr := doPostForm(t, mux, adminRecordsBase(ent)+"/new", fullRecordForm(), admin)
	if !strings.Contains(location(t, rr), "successfully") {
		t.Fatalf("first create failed: %s", location(t, rr))
	}

	dup := fullRecordForm()
	dup.Set("attr_titulo", "Outro")
	rr = doPostForm(t, mux, adminRecordsBase(ent)+"/new", dup, admin)
	loc := location(t, rr)
	if !strings.Contains(loc, "already exists") {
		t.Fatalf("want unique violation, got: %s", loc)
	}
}

func TestAdminRecordPreSaveBlocks(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, `(set error "blocked by script")`)
	admin := plantUser(t, "admin", true)

	rr := doPostForm(t, mux, adminRecordsBase(ent)+"/new", fullRecordForm(), admin)
	loc := location(t, rr)
	if !strings.Contains(loc, "blocked") {
		t.Fatalf("want pre_save block, got: %s", loc)
	}
	records, err := s.ListEAVRecordsCursor(ent.et.ID, 0, 10, "")
	if err != nil {
		t.Fatalf("ListEAVRecordsCursor: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("record persisted despite pre_save block")
	}
}

func TestAdminRecordEditPageRenders(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, "")
	admin := plantUser(t, "admin", true)

	doPostForm(t, mux, adminRecordsBase(ent)+"/new", fullRecordForm(), admin)
	rec, _ := recordValues(t, s, ent)

	path := adminRecordsBase(ent) + "/" + rec.ReferenceID + "/edit"
	rr := doGet(t, mux, path, admin)
	assertRendered(t, rr, path)
	if !strings.Contains(rr.Body.String(), "Comprar") {
		t.Fatalf("edit page does not show the record values")
	}
}

func TestAdminRecordUpdateHappyPath(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, "")
	admin := plantUser(t, "admin", true)

	doPostForm(t, mux, adminRecordsBase(ent)+"/new", fullRecordForm(), admin)
	rec, _ := recordValues(t, s, ent)

	form := fullRecordForm()
	form.Set("attr_titulo", "Atualizado")
	form.Set("attr_feito", "0")
	form.Set("rev", "2") // create inserts rev=1 and activation bumps to 2

	rr := doPostForm(t, mux, adminRecordsBase(ent)+"/"+rec.ReferenceID+"/update", form, admin)
	loc := location(t, rr)
	if !strings.Contains(loc, "updated successfully") {
		t.Fatalf("update redirected with: %s", loc)
	}

	updated, vals := recordValues(t, s, ent)
	if vals["titulo"] != "Atualizado" {
		t.Fatalf("titulo = %v", vals["titulo"])
	}
	if vals["feito"] != false {
		t.Fatalf("feito = %v", vals["feito"])
	}
	if updated.Status != "active" {
		t.Fatalf("status = %q", updated.Status)
	}
	if updated.Rev <= rec.Rev {
		t.Fatalf("rev not bumped: %d -> %d", rec.Rev, updated.Rev)
	}
}

func TestAdminRecordUpdateStaleRevConflict(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, "")
	admin := plantUser(t, "admin", true)

	doPostForm(t, mux, adminRecordsBase(ent)+"/new", fullRecordForm(), admin)
	rec, _ := recordValues(t, s, ent)

	form := fullRecordForm()
	form.Set("rev", "1") // stale: record is at rev 2 after activation

	rr := doPostForm(t, mux, adminRecordsBase(ent)+"/"+rec.ReferenceID+"/update", form, admin)
	loc := location(t, rr)
	if !strings.Contains(loc, "Conflict") {
		t.Fatalf("want optimistic-lock conflict, got: %s", loc)
	}
}

func TestAdminRecordDelete(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, "")
	admin := plantUser(t, "admin", true)

	doPostForm(t, mux, adminRecordsBase(ent)+"/new", fullRecordForm(), admin)
	rec, _ := recordValues(t, s, ent)

	rr := doPostForm(t, mux, adminRecordsBase(ent)+"/"+rec.ReferenceID+"/delete", url.Values{}, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("delete = %d, want 303", rr.Code)
	}

	records, err := s.ListEAVRecordsCursor(ent.et.ID, 0, 10, "")
	if err != nil {
		t.Fatalf("ListEAVRecordsCursor: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("record still listed after soft delete")
	}
}

func TestAdminRecordsPageAndAPI(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, "")
	admin := plantUser(t, "admin", true)

	doPostForm(t, mux, adminRecordsBase(ent)+"/new", fullRecordForm(), admin)

	rr := doGet(t, mux, adminRecordsBase(ent), admin)
	assertRendered(t, rr, adminRecordsBase(ent))
	if !strings.Contains(rr.Body.String(), "Comprar") {
		t.Fatalf("records page does not show the record")
	}

	rr = doGet(t, mux, adminRecordsBase(ent)+"/api?offset=0", admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("records api = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"total\":1") {
		t.Fatalf("records api body: %.200s", rr.Body.String())
	}
}

// FormsRuntimeEdit is the end-user record editor; it must render stored values.
func TestFormsRuntimeEditRendersValues(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form := seedTaskForm(t, s, "Comprar leite")
	user := plantUser(t, "user", false)

	records, err := s.ListEAVRecordsCursor(*form.EAVEntityTypeID, 0, 10, "")
	if err != nil || len(records) == 0 {
		t.Fatalf("seed record missing: %v", err)
	}

	path := "/form/" + form.MachineName + "/r/" + records[0].ReferenceID
	rr := doGet(t, mux, path, user)
	assertRendered(t, rr, path)
	if !strings.Contains(rr.Body.String(), "Comprar leite") {
		t.Fatalf("runtime edit does not show the stored value")
	}
}

// Regression: the "Maximum: N characters" hint on a TEXT field with a
// max_length must render the value, not the pointer address. The bug
// (dogfood, QA Fase 4) passed *int to the i18n %d verb, printing the
// address — a number that changed between renders.
func TestAdminRecordMaxLengthHint(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	ent := seedAdminEntity(t, s, "")
	admin := plantUser(t, "admin", true)

	rr := doGet(t, mux, adminRecordsBase(ent)+"/new", admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("record new = %d", rr.Code)
	}
	body := rr.Body.String()
	// titulo has max_length 10.
	if !strings.Contains(body, "Maximum: 10 characters") &&
		!strings.Contains(body, "Máximo: 10 caracteres") {
		t.Fatalf("max length hint not rendered as the value 10: %s",
			body[max(0, strings.Index(body, "aximum")-10):min(len(body), strings.Index(body, "aximum")+40)])
	}
	// A pointer address renders as a huge number (>= 6 digits); guard
	// against the regression returning.
	if regexp.MustCompile(`(?:Maximum|Máximo): \d{6,}`).MatchString(body) {
		t.Fatal("max length hint looks like a pointer address")
	}
}
