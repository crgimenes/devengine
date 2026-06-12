package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/db"
)

// seedSearchEntity creates an entity type with a single TEXT attribute and
// one record per given value.
func seedSearchEntity(t *testing.T, s db.Store, name, machine string, values ...string) *db.EAVEntityType {
	t.Helper()
	et, err := s.CreateEAVEntityType(name, machine, "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType(%s): %v", machine, err)
	}
	attr, err := s.CreateEAVAttribute(et.ID, "nome", "Nome", "", "TEXT",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(%s): %v", machine, err)
	}
	for _, v := range values {
		rec, err := s.CreateEAVRecord(et.ID)
		if err != nil {
			t.Fatalf("CreateEAVRecord(%s): %v", machine, err)
		}
		val := v
		err = s.UpsertEAVValue(rec.ID, attr.ID, nil, nil, nil, &val, nil)
		if err != nil {
			t.Fatalf("UpsertEAVValue(%s): %v", machine, err)
		}
	}
	return et
}

func TestGlobalSearchGroupsAcrossEntities(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	clientes := seedSearchEntity(t, s, "Cliente", "cliente", "Ana Souza", "Bruno Lima")
	seedSearchEntity(t, s, "Produto", "produto", "Teclado da Ana", "Mouse")
	seedSearchEntity(t, s, "Vazio", "vazio")

	rr := doGet(t, mux, "/tools/search-forms?q=Ana", admin)
	assertRendered(t, rr, "/tools/search-forms?q=Ana")
	body := rr.Body.String()

	for _, want := range []string{"Cliente", "Produto", "Ana Souza", "Teclado da Ana"} {
		if !strings.Contains(body, want) {
			t.Fatalf("%q missing from search results", want)
		}
	}
	if strings.Contains(body, "Vazio") {
		t.Fatal("entity without matches must be omitted")
	}
	if strings.Contains(body, "Bruno Lima") {
		t.Fatal("non-matching record leaked into results")
	}
	if !strings.Contains(body, "/tools/database-schema/eav/"+clientes.ReferenceID+"/records/") {
		t.Fatal("result hit missing record edit link")
	}
}

func TestGlobalSearchNoMatches(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	seedSearchEntity(t, s, "Cliente", "cliente", "Ana Souza")

	rr := doGet(t, mux, "/tools/search-forms?q=zzz-nada", admin)
	assertRendered(t, rr, "/tools/search-forms?q=zzz-nada")
	if !strings.Contains(rr.Body.String(), "No records match") {
		t.Fatalf("empty-result message missing: %.300s", rr.Body.String())
	}
}

func TestGlobalSearchEmptyQueryShowsForm(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	rr := doGet(t, mux, "/tools/search-forms", admin)
	assertRendered(t, rr, "/tools/search-forms")
	body := rr.Body.String()
	if !strings.Contains(body, `name="q"`) {
		t.Fatal("search input missing")
	}
	if strings.Contains(body, "No records match") {
		t.Fatal("empty query must not claim zero results")
	}
}

func TestGlobalSearchForbiddenForNonSysop(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	user := plantUser(t, "user", false)

	rr := doGet(t, mux, "/tools/search-forms?q=x", user)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-sysop search = %d, want 403", rr.Code)
	}
}

// CSV export must resolve reference columns to the display label and keep
// the raw id in a sibling "<name>_ref" column.
func TestCSVExportResolvesReferenceDisplay(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	clientes := seedSearchEntity(t, s, "Cliente", "cliente", "Ana Souza")
	recs, err := s.ListEAVRecordsCursor(clientes.ID, 0, 1, "")
	if err != nil || len(recs) != 1 {
		t.Fatalf("cliente record: %v (%d)", err, len(recs))
	}
	ana := recs[0]

	pedidos, err := s.CreateEAVEntityType("Pedido", "pedido_csv", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	cliRef, err := s.CreateEAVAttribute(pedidos.ID, "cliente", "Cliente", "", "TEXT",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute: %v", err)
	}
	form, err := s.CreateForm("pedido_csv", "Pedido", "", &pedidos.ID)
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
		t.Fatalf("CreateEAVRecord: %v", err)
	}
	err = s.UpsertEAVValue(rec.ID, cliRef.ID, nil, nil, nil, &ana.ReferenceID, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue: %v", err)
	}

	rr := doGet(t, mux, "/tools/forms/"+form.ReferenceID+"/records/export.csv", admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("export = %d", rr.Code)
	}
	csvBody := rr.Body.String()
	lines := strings.Split(strings.TrimSpace(csvBody), "\n")
	if len(lines) != 2 {
		t.Fatalf("csv lines = %d: %q", len(lines), csvBody)
	}
	if !strings.Contains(lines[0], "cliente,cliente_ref") {
		t.Fatalf("header missing display+ref pair: %q", lines[0])
	}
	if !strings.Contains(lines[1], "Ana Souza") {
		t.Fatalf("row missing display label: %q", lines[1])
	}
	if !strings.Contains(lines[1], ana.ReferenceID) {
		t.Fatalf("row missing raw ref id: %q", lines[1])
	}
}

// A form flagged as search must answer its listing (search box + rows) on
// GET /form/{name}, and each form's listing shows only its own fields.
func TestSearchFormOpensOnListingWithOwnColumns(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	user := plantUser(t, "user", false)

	et, err := s.CreateEAVEntityType("Pedido", "pedido_busca", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	mk := func(machine, label string) *db.EAVAttribute {
		a, err := s.CreateEAVAttribute(et.ID, machine, label, "", "TEXT",
			false, false, false, nil, false, "", nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateEAVAttribute(%s): %v", machine, err)
		}
		return a
	}
	cliente := mk("cliente", "ColCliente")
	destino := mk("destino", "ColDestino")

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord: %v", err)
	}
	vc, vd := "Ana Souza", "Curitiba"
	for _, pair := range []struct {
		attr *db.EAVAttribute
		val  *string
	}{{cliente, &vc}, {destino, &vd}} {
		err = s.UpsertEAVValue(rec.ID, pair.attr.ID, nil, nil, nil, pair.val, nil)
		if err != nil {
			t.Fatalf("UpsertEAVValue: %v", err)
		}
	}

	// Search form exposing ONLY the cliente field.
	form, err := s.CreateForm("busca_cliente", "Busca por cliente", "", &et.ID)
	if err != nil {
		t.Fatalf("CreateForm: %v", err)
	}
	_, err = s.CreateFormElement(form.ID, nil, "cliente", "field", "ColCliente",
		"", 0, 12, "", "", &cliente.ID, false, false)
	if err != nil {
		t.Fatalf("CreateFormElement: %v", err)
	}
	err = s.UpdateForm(form.ID, form.MachineName, form.Label, "", &et.ID,
		false, false, false, false, nil, true, false)
	if err != nil {
		t.Fatalf("UpdateForm: %v", err)
	}

	rr := doGet(t, mux, "/form/busca_cliente", user)
	assertRendered(t, rr, "/form/busca_cliente")
	body := rr.Body.String()
	if !strings.Contains(body, `name="q"`) {
		t.Fatal("search form did not open on the listing (no search box)")
	}
	if !strings.Contains(body, "Ana Souza") {
		t.Fatal("listing missing record data")
	}
	if !strings.Contains(body, "ColCliente") {
		t.Fatal("listing missing the form's own column")
	}
	if strings.Contains(body, "ColDestino") {
		t.Fatal("listing leaked a column the form does not expose")
	}

	// A second search form over the SAME entity is a different view.
	form2, err := s.CreateForm("busca_destino", "Busca por destino", "", &et.ID)
	if err != nil {
		t.Fatalf("CreateForm 2: %v", err)
	}
	_, err = s.CreateFormElement(form2.ID, nil, "destino", "field", "ColDestino",
		"", 0, 12, "", "", &destino.ID, false, false)
	if err != nil {
		t.Fatalf("CreateFormElement 2: %v", err)
	}
	err = s.UpdateForm(form2.ID, form2.MachineName, form2.Label, "", &et.ID,
		false, false, false, false, nil, true, false)
	if err != nil {
		t.Fatalf("UpdateForm 2: %v", err)
	}

	rr = doGet(t, mux, "/form/busca_destino", user)
	assertRendered(t, rr, "/form/busca_destino")
	body = rr.Body.String()
	if !strings.Contains(body, "ColDestino") || !strings.Contains(body, "Curitiba") {
		t.Fatal("second view missing its own column/data")
	}
	if strings.Contains(body, "ColCliente") {
		t.Fatal("second view leaked the first view's column")
	}
}

// A regular (non-search) form keeps opening on the create view.
func TestRegularFormStillOpensOnCreateView(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	user := plantUser(t, "user", false)
	form := seedTaskForm(t, s, "seed")

	rr := doGet(t, mux, "/form/"+form.MachineName, user)
	assertRendered(t, rr, "/form/"+form.MachineName)
	if !strings.Contains(rr.Body.String(), "<form") {
		t.Fatal("regular form did not render the create view")
	}
}

// Creating a menu item must land on the item editor, not back on the menu:
// a fresh item is a bare link and always needs more configuration.
func TestMenuItemCreateOpensItemEditor(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	menu, err := s.CreateMenu("principal", "Principal", "")
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	// Menu mutations validate the double-submit CSRF pair.
	csrf := &http.Cookie{Name: "csrf", Value: "test-csrf-token"}
	body := url.Values{
		"machine_name": {"docs"},
		"label":        {"Documentos"},
		"csrf_token":   {csrf.Value},
	}
	req := httptest.NewRequest(http.MethodPost,
		"/tools/menu-editor/"+menu.ReferenceID+"/items/new",
		strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(admin)
	req.AddCookie(csrf)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("item create = %d, body: %.300s", rr.Code, rr.Body.String())
	}
	loc := location(t, rr)
	if !strings.Contains(loc, "/items/") || !strings.Contains(loc, "/edit") {
		t.Fatalf("create redirected to %q, want the item editor", loc)
	}

	// Re-GET only the path: the ?message= part carries spaces that
	// httptest.NewRequest rejects (browsers tolerate them).
	path := strings.SplitN(strings.TrimPrefix(loc, "http://localhost:3210"), "?", 2)[0]
	rr = doGet(t, mux, path, admin)
	assertRendered(t, rr, path)
	if !strings.Contains(rr.Body.String(), "docs") {
		t.Fatalf("item editor missing the new item: %.300s", rr.Body.String())
	}
}

// The element editor must offer the reference/subform option panels with
// the datalists of real entity and attribute machine names.
func TestElementEditOffersReferencePanel(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	seedSearchEntity(t, s, "Cliente", "cliente", "Ana Souza")
	form := seedTaskForm(t, s, "seed")

	els, err := s.ListFormElements(form.ID)
	if err != nil || len(els) == 0 {
		t.Fatalf("ListFormElements: %v", err)
	}

	rr := doGet(t, mux,
		"/tools/forms/"+form.ReferenceID+"/elements/"+els[0].ReferenceID+"/edit", admin)
	assertRendered(t, rr, "/element edit")
	body := rr.Body.String()
	for _, want := range []string{
		`id="field-options-reference"`,
		`id="field-options-subform"`,
		`data-key="entity"`,
		`data-key="target_entity"`,
		`id="entity-machine-names"`,
		`value="cliente"`, // real entity in the datalist
		`value="nome"`,    // real attribute in the datalist
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("%s missing from element editor", want)
		}
	}
}
