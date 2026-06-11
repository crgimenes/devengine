package handlers

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/db"
)

// Authoring a validate_expr through the element editor must persist it and the
// runtime must enforce it. The execution side existed but had no authoring UI
// and ListFormElements did not even load the column.
func TestElementValidateExprAuthoringAndRuntime(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form := seedTaskForm(t, s, "seed")
	admin := plantUser(t, "admin", true)
	user := plantUser(t, "user", false)

	els, err := s.ListFormElements(form.ID)
	if err != nil || len(els) == 0 {
		t.Fatalf("ListFormElements: %v (%d)", err, len(els))
	}
	el := els[0]

	attrs, err := s.ListEAVAttributesByEntityTypeID(*form.EAVEntityTypeID)
	if err != nil || len(attrs) == 0 {
		t.Fatalf("ListEAVAttributesByEntityTypeID: %v", err)
	}

	expr := `(if (= field:titulo "proibido") "valor proibido" "")`
	body := url.Values{
		"machine_name":     {el.MachineName},
		"element_kind":     {el.ElementKind},
		"label":            {el.Label},
		"z_order":          {"0"},
		"col_span":         {"12"},
		"alignment":        {"left"},
		"eav_attribute_id": {attrs[0].ReferenceID},
		"validate_expr":    {expr},
	}
	rr := doPostForm(t, mux,
		"/tools/forms/"+form.ReferenceID+"/elements/"+el.ReferenceID+"/update", body, admin)
	if rr.Code != 303 && rr.Code != 200 {
		t.Fatalf("element update = %d", rr.Code)
	}

	got, err := s.GetFormElementByRefID(el.ReferenceID)
	if err != nil {
		t.Fatalf("GetFormElementByRefID: %v", err)
	}
	if got.ValidateExpr != expr {
		t.Fatalf("validate_expr not persisted: %q", got.ValidateExpr)
	}

	// Runtime must block the forbidden value, re-rendering the form with the
	// message next to the field and the typed value preserved.
	rr = doPostForm(t, mux, "/form/"+form.MachineName,
		url.Values{"titulo": {"proibido"}}, user)
	if rr.Code != 200 {
		t.Fatalf("invalid submit = %d, want re-render", rr.Code)
	}
	page := rr.Body.String()
	if !strings.Contains(page, "valor proibido") {
		t.Fatalf("validate_expr message missing from re-render")
	}
	if !strings.Contains(page, `value="proibido"`) {
		t.Fatalf("typed value not preserved on re-render")
	}

	// ...and accept anything else.
	rr = doPostForm(t, mux, "/form/"+form.MachineName,
		url.Values{"titulo": {"permitido"}}, user)
	loc := location(t, rr)
	if !strings.Contains(loc, "successfully") {
		t.Fatalf("valid value rejected, redirect: %s", loc)
	}
}

// Authoring a computed attribute through the attribute editor must persist
// is_computed/computed_expr and the runtime must overwrite the field on save.
func TestAttributeComputedAuthoringAndRuntime(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	user := plantUser(t, "user", false)

	et, err := s.CreateEAVEntityType("Venda", "venda", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	mk := func(machine, kind string) db.EAVAttribute {
		a, err := s.CreateEAVAttribute(et.ID, machine, machine, "", kind,
			false, false, false, nil, false, "", nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateEAVAttribute(%s): %v", machine, err)
		}
		return *a
	}
	valor := mk("valor", "REAL")
	quantidade := mk("quantidade", "INT")
	total := mk("total", "REAL")

	form, err := s.CreateForm("venda", "Venda", "", &et.ID)
	if err != nil {
		t.Fatalf("CreateForm: %v", err)
	}
	for i, a := range []db.EAVAttribute{valor, quantidade} {
		_, err = s.CreateFormElement(form.ID, nil, a.MachineName, "field", a.MachineName,
			"", i, 12, "", "", &a.ID, false, false)
		if err != nil {
			t.Fatalf("CreateFormElement(%s): %v", a.MachineName, err)
		}
	}

	expr := `(* field:valor field:quantidade)`
	body := url.Values{
		"machine_name":   {"total"},
		"label":          {"total"},
		"primitive_kind": {"REAL"},
		"is_computed":    {"1"},
		"computed_expr":  {expr},
	}
	rr := doPostForm(t, mux,
		"/tools/database-schema/eav/"+et.ReferenceID+"/attributes/"+total.ReferenceID+"/update",
		body, admin)
	if rr.Code != 303 {
		t.Fatalf("attribute update = %d: %.200s", rr.Code, rr.Body.String())
	}

	got, err := s.GetEAVAttributeByRefID(total.ReferenceID)
	if err != nil {
		t.Fatalf("GetEAVAttributeByRefID: %v", err)
	}
	if !got.IsComputed || got.ComputedExpr != expr {
		t.Fatalf("computed not persisted: computed=%v expr=%q", got.IsComputed, got.ComputedExpr)
	}

	rr = doPostForm(t, mux, "/form/venda",
		url.Values{"valor": {"2.5"}, "quantidade": {"4"}}, user)
	loc := location(t, rr)
	if !strings.Contains(loc, "successfully") {
		t.Fatalf("runtime create failed: %s", loc)
	}

	records, err := s.ListEAVRecordsCursor(et.ID, 0, 10, "")
	if err != nil || len(records) != 1 {
		t.Fatalf("records: %v (%d)", err, len(records))
	}
	vals, err := s.GetEAVValuesByRecordID(records[0].ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID: %v", err)
	}
	var gotTotal *float64
	for _, v := range vals {
		if v.AttributeID == total.ID {
			gotTotal = v.VReal
		}
	}
	if gotTotal == nil || *gotTotal != 10.0 {
		t.Fatalf("total = %v, want 10.0", gotTotal)
	}
}

// Listings must show the referenced record's display value, not the raw id.
func TestListingResolvesReferenceDisplay(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	user := plantUser(t, "user", false)

	clientes, err := s.CreateEAVEntityType("Cliente", "cliente", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType(cliente): %v", err)
	}
	nome, err := s.CreateEAVAttribute(clientes.ID, "nome", "Nome", "", "TEXT",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(nome): %v", err)
	}
	ana, err := s.CreateEAVRecord(clientes.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord(ana): %v", err)
	}
	anaNome := "Ana Souza"
	err = s.UpsertEAVValue(ana.ID, nome.ID, nil, nil, nil, &anaNome, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue: %v", err)
	}

	pedidos, err := s.CreateEAVEntityType("Pedido", "pedido2", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType(pedido2): %v", err)
	}
	cliRef, err := s.CreateEAVAttribute(pedidos.ID, "cliente", "Cliente", "", "TEXT",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(cliente): %v", err)
	}
	form, err := s.CreateForm("pedido2", "Pedido", "", &pedidos.ID)
	if err != nil {
		t.Fatalf("CreateForm: %v", err)
	}
	el, err := s.CreateFormElement(form.ID, nil, "cliente", "field", "Cliente",
		"", 0, 12, "", "", &cliRef.ID, false, false)
	if err != nil {
		t.Fatalf("CreateFormElement: %v", err)
	}
	err = s.UpdateFormElement(el.ID, nil, "cliente", "field", "Cliente", "",
		0, 12, "left", "reference",
		`{"entity": "cliente", "display": "nome"}`,
		&cliRef.ID, false, false, false, false,
		"", "", false, "", "", "")
	if err != nil {
		t.Fatalf("UpdateFormElement: %v", err)
	}

	rec, err := s.CreateEAVRecord(pedidos.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord(pedido): %v", err)
	}
	err = s.UpsertEAVValue(rec.ID, cliRef.ID, nil, nil, nil, &ana.ReferenceID, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue(ref): %v", err)
	}

	rr := doGet(t, mux, "/form/pedido2/list", user)
	assertRendered(t, rr, "/form/pedido2/list")
	body := rr.Body.String()
	if !strings.Contains(body, "Ana Souza") {
		t.Fatalf("listing does not resolve reference display")
	}
	if strings.Contains(body, ana.ReferenceID) {
		t.Fatalf("listing still leaks the raw reference id")
	}
}

// seedDatetimeForm builds an entity with a single DATETIME attribute carrying
// the given default, plus a bound form+element.
func seedDatetimeForm(t *testing.T, s *db.SQLite, defaultValue string) *db.Form {
	t.Helper()
	et, err := s.CreateEAVEntityType("Evento", "evento", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	attr, err := s.CreateEAVAttribute(et.ID, "data", "Data", "", "DATETIME",
		false, false, false, nil, false, "", nil, nil, nil, nil, &defaultValue)
	if err != nil {
		t.Fatalf("CreateEAVAttribute: %v", err)
	}
	form, err := s.CreateForm("evento", "Evento", "", &et.ID)
	if err != nil {
		t.Fatalf("CreateForm: %v", err)
	}
	_, err = s.CreateFormElement(form.ID, nil, "data", "field", "Data",
		"", 0, 12, "", "", &attr.ID, false, false)
	if err != nil {
		t.Fatalf("CreateFormElement: %v", err)
	}
	return form
}

// A static datetime default must arrive pre-filled in the create form.
func TestDatetimeDefaultPrefillsForm(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	seedDatetimeForm(t, s, "2026-06-15T18:00")
	user := plantUser(t, "user", false)

	rr := doGet(t, mux, "/form/evento", user)
	assertRendered(t, rr, "/form/evento")
	if !strings.Contains(rr.Body.String(), `value="2026-06-15T18:00"`) {
		t.Fatalf("datetime default not pre-filled")
	}
}

// The special default "now" must render as the current timestamp.
func TestDatetimeDefaultNow(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	seedDatetimeForm(t, s, "now")
	user := plantUser(t, "user", false)

	rr := doGet(t, mux, "/form/evento", user)
	assertRendered(t, rr, "/form/evento")
	body := rr.Body.String()
	if strings.Contains(body, `value="now"`) || strings.Contains(body, `value=""`+` name="data"`) {
		t.Fatalf("'now' default not resolved")
	}
	m := regexp.MustCompile(`name="data"\s+value="(\d{4}-\d{2}-\d{2}T\d{2}:\d{2})"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("data input not filled with a timestamp")
	}
}

// A parse error (bad INT) must re-render with the message on the offending
// field and every typed value preserved — including the bad one.
func TestCreateParseErrorPreservesValues(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form, _ := seedStockScenario(t, s)
	user := plantUser(t, "user", false)

	rr := doPostForm(t, mux, "/form/"+form.MachineName,
		url.Values{"quantidade": {"abc"}, "produto": {"manter isto"}}, user)
	if rr.Code != 200 {
		t.Fatalf("bad submit = %d, want re-render", rr.Code)
	}
	page := rr.Body.String()
	if !strings.Contains(page, "invalid value") {
		t.Fatalf("parse error message missing")
	}
	if !strings.Contains(page, `value="abc"`) {
		t.Fatalf("bad input not preserved for correction")
	}
	if !strings.Contains(page, `value="manter isto"`) {
		t.Fatalf("good input lost on re-render")
	}

	records, err := s.ListEAVRecordsCursor(*form.EAVEntityTypeID, 0, 10, "")
	if err != nil || len(records) != 0 {
		t.Fatalf("records = %d (%v), want 0", len(records), err)
	}
}

// Update validation failures must re-render the edit view (rev hidden intact)
// without touching the stored record.
func TestUpdateValidationRerendersWithValues(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form := seedTaskForm(t, s, "original")
	user := plantUser(t, "user", false)

	els, err := s.ListFormElements(form.ID)
	if err != nil || len(els) == 0 {
		t.Fatalf("ListFormElements: %v", err)
	}
	el := els[0]
	err = s.UpdateFormElement(el.ID, nil, el.MachineName, el.ElementKind, el.Label, "",
		0, 12, "left", "", "", el.EAVAttributeID, false, false, false, false,
		`(if (= field:titulo "proibido") "valor proibido" "")`,
		"", false, "", "", "")
	if err != nil {
		t.Fatalf("UpdateFormElement: %v", err)
	}

	records, err := s.ListEAVRecordsCursor(*form.EAVEntityTypeID, 0, 10, "")
	if err != nil || len(records) != 1 {
		t.Fatalf("seed record: %v", err)
	}
	rec := records[0]

	rr := doPostForm(t, mux, "/form/"+form.MachineName+"/r/"+rec.ReferenceID,
		url.Values{"titulo": {"proibido"}, "rev": {strconv.Itoa(rec.Rev)}}, user)
	if rr.Code != 200 {
		t.Fatalf("invalid update = %d, want re-render", rr.Code)
	}
	page := rr.Body.String()
	if !strings.Contains(page, "valor proibido") || !strings.Contains(page, `value="proibido"`) {
		t.Fatalf("re-render missing error or typed value")
	}

	vals, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil || len(vals) != 1 || vals[0].VText == nil || *vals[0].VText != "original" {
		t.Fatalf("stored value changed: %+v", vals)
	}
}
