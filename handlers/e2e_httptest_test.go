package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/auth/basic"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/templates"
)

// TestEndToEndFlow walks the whole engine through HTTP, the way a person
// would: real login, then authoring (entity type → attributes → form →
// elements with a validate_expr) and finally usage (runtime create blocked
// by validation, then accepted, then visible on the listing). It is the
// executable narrative of roadmap item 7.1.
func TestEndToEndFlow(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	basicAuth := basic.New(nil, templates.ExecuteTemplate)
	basicAuth.Routes(mux)

	// --- login (the real one, password and all) ---
	hash, err := basic.HashPassword("s3nha-forte")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	_, err = db.Storage.CreateUser("root", "root@example.com", hash, true)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	rr := doPostForm(t, mux, "/login",
		url.Values{"username": {"root"}, "password": {"s3nha-forte"}}, nil)
	if rr.Code != http.StatusFound {
		t.Fatalf("login = %d: %.300s", rr.Code, rr.Body.String())
	}
	var sid *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "sid" && c.Value != "" {
			sid = c
		}
	}
	if sid == nil {
		t.Fatal("login did not set a session cookie")
	}

	// --- authoring: entity type ---
	rr = doPostForm(t, mux, "/tools/database-schema/eav/new", url.Values{
		"name": {"Chamado"}, "machine_name": {"chamado"},
	}, sid)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("entity create = %d: %.300s", rr.Code, rr.Body.String())
	}
	etRef := regexp.MustCompile(`/eav/([^/]+)/edit`).FindStringSubmatch(location(t, rr))
	if etRef == nil {
		t.Fatalf("entity redirect without ref: %s", location(t, rr))
	}

	// --- authoring: attributes ---
	for _, a := range []url.Values{
		{"machine_name": {"assunto"}, "label": {"Assunto"}, "primitive_kind": {"TEXT"}, "is_required": {"1"}},
		{"machine_name": {"prioridade"}, "label": {"Prioridade"}, "primitive_kind": {"INT"}},
	} {
		rr = doPostForm(t, mux, "/tools/database-schema/eav/"+etRef[1]+"/attributes/new", a, sid)
		if rr.Code != http.StatusSeeOther {
			t.Fatalf("attribute create = %d: %.300s", rr.Code, rr.Body.String())
		}
	}

	// --- authoring: form bound to the entity ---
	rr = doPostForm(t, mux, "/tools/forms/new", url.Values{
		"machine_name": {"abrir_chamado"}, "label": {"Abrir Chamado"},
		"entity_type_id": {etRef[1]},
	}, sid)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("form create = %d: %.300s", rr.Code, rr.Body.String())
	}
	formRef := regexp.MustCompile(`/forms/([^/]+)/edit`).FindStringSubmatch(location(t, rr))
	if formRef == nil {
		t.Fatalf("form redirect without ref: %s", location(t, rr))
	}
	form, err := db.Storage.GetFormByRefID(formRef[1])
	if err != nil {
		t.Fatalf("GetFormByRefID: %v", err)
	}

	// --- authoring: one element per attribute ---
	attrs, err := db.Storage.ListEAVAttributesByEntityTypeID(*form.EAVEntityTypeID)
	if err != nil || len(attrs) != 2 {
		t.Fatalf("attributes: %v (%d)", err, len(attrs))
	}
	attrRef := map[string]string{}
	for _, a := range attrs {
		attrRef[a.MachineName] = a.ReferenceID
	}
	for _, machine := range []string{"assunto", "prioridade"} {
		rr = doPostForm(t, mux, "/tools/forms/"+formRef[1]+"/elements/new", url.Values{
			"machine_name": {machine}, "element_kind": {"field"}, "label": {machine},
			"eav_attribute_id": {attrRef[machine]},
		}, sid)
		if rr.Code != http.StatusSeeOther {
			t.Fatalf("element create (%s) = %d", machine, rr.Code)
		}
	}

	// --- authoring: validate_expr on assunto ---
	els, err := db.Storage.ListFormElements(form.ID)
	if err != nil || len(els) != 2 {
		t.Fatalf("elements: %v (%d)", err, len(els))
	}
	el := els[0]
	rr = doPostForm(t, mux,
		"/tools/forms/"+formRef[1]+"/elements/"+el.ReferenceID+"/update", url.Values{
			"machine_name": {el.MachineName}, "element_kind": {el.ElementKind},
			"label": {el.Label}, "z_order": {"0"}, "col_span": {"12"},
			"alignment":        {"left"},
			"eav_attribute_id": {attrRef[el.MachineName]},
			"validate_expr":    {`(if (= field:assunto "spam") "assunto proibido" "")`},
		}, sid)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK {
		t.Fatalf("element update = %d", rr.Code)
	}

	// --- usage: validation blocks and preserves the typed values ---
	rr = doPostForm(t, mux, "/form/abrir_chamado",
		url.Values{"assunto": {"spam"}, "prioridade": {"3"}}, sid)
	if rr.Code != http.StatusOK {
		t.Fatalf("invalid submit = %d, want re-render", rr.Code)
	}
	page := rr.Body.String()
	if !strings.Contains(page, "assunto proibido") || !strings.Contains(page, `value="spam"`) {
		t.Fatalf("validation re-render missing message or value: %.300s", page)
	}

	// --- usage: a valid record goes through ---
	rr = doPostForm(t, mux, "/form/abrir_chamado",
		url.Values{"assunto": {"Impressora pegou fogo"}, "prioridade": {"1"}}, sid)
	loc := location(t, rr)
	if !strings.Contains(loc, "successfully") {
		t.Fatalf("valid submit rejected: %s", loc)
	}

	// --- usage: the record shows on the listing ---
	rr = doGet(t, mux, "/form/abrir_chamado/list", sid)
	assertRendered(t, rr, "/form/abrir_chamado/list")
	if !strings.Contains(rr.Body.String(), "Impressora pegou fogo") {
		t.Fatalf("record missing from listing: %.300s", rr.Body.String())
	}

	// --- authoring: a menu bound to the form shows up in the runtime ---
	rr = doPostCSRF(t, mux, "/tools/menu-editor/new", url.Values{
		"machine_name": {"suporte"}, "label": {"Suporte"},
	}, sid)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
		t.Fatalf("menu create = %d", rr.Code)
	}
	menu, err := db.Storage.GetMenuByMachineName("suporte")
	if err != nil || menu == nil {
		t.Fatalf("menu not created: %v", err)
	}
	rr = doPostCSRF(t, mux, "/tools/menu-editor/"+menu.ReferenceID+"/items/new", url.Values{
		"machine_name": {"painel"}, "label": {"Painel de Chamados"},
	}, sid)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
		t.Fatalf("menu item create = %d", rr.Code)
	}
	rr = doPostForm(t, mux, "/tools/forms/"+formRef[1]+"/update", url.Values{
		"machine_name": {"abrir_chamado"}, "label": {"Abrir Chamado"},
		"entity_type_id": {etRef[1]},
		"menu_id":        {menu.ReferenceID},
	}, sid)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("form bind menu = %d", rr.Code)
	}
	rr = doGet(t, mux, "/form/abrir_chamado", sid)
	assertRendered(t, rr, "/form/abrir_chamado with menu")
	if !strings.Contains(rr.Body.String(), "Painel de Chamados") {
		t.Fatalf("bound menu missing from runtime page: %.300s", rr.Body.String())
	}

	// --- authoring: a search form over the same entity ---
	rr = doPostForm(t, mux, "/tools/forms/new", url.Values{
		"machine_name": {"busca_chamados"}, "label": {"Busca de Chamados"},
		"entity_type_id": {etRef[1]},
	}, sid)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("search form create = %d", rr.Code)
	}
	searchRef := regexp.MustCompile(`/forms/([^/]+)/edit`).FindStringSubmatch(location(t, rr))
	if searchRef == nil {
		t.Fatalf("search form redirect without ref: %s", location(t, rr))
	}
	rr = doPostForm(t, mux, "/tools/forms/"+searchRef[1]+"/elements/new", url.Values{
		"machine_name": {"assunto"}, "element_kind": {"field"}, "label": {"Assunto"},
		"eav_attribute_id": {attrRef["assunto"]},
	}, sid)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("search form element = %d", rr.Code)
	}
	rr = doPostForm(t, mux, "/tools/forms/"+searchRef[1]+"/update", url.Values{
		"machine_name": {"busca_chamados"}, "label": {"Busca de Chamados"},
		"entity_type_id": {etRef[1]},
		"is_search":      {"on"},
	}, sid)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("search flag update = %d", rr.Code)
	}

	// Opening the search form lands on the listing with the record, and the
	// text filter narrows it.
	rr = doGet(t, mux, "/form/busca_chamados", sid)
	assertRendered(t, rr, "/form/busca_chamados")
	if !strings.Contains(rr.Body.String(), "Impressora pegou fogo") {
		t.Fatalf("search form listing missing record: %.300s", rr.Body.String())
	}
	rr = doGet(t, mux, "/form/busca_chamados/list?q=impressora", sid)
	assertRendered(t, rr, "search with filter")
	if !strings.Contains(rr.Body.String(), "Impressora pegou fogo") {
		t.Fatal("filter missed the record")
	}
	rr = doGet(t, mux, "/form/busca_chamados/list?q=geladeira", sid)
	if strings.Contains(rr.Body.String(), "Impressora pegou fogo") {
		t.Fatal("filter matched a record it should not")
	}

	// --- relationships: a reference field resolves its display value ---
	rr = doPostForm(t, mux, "/tools/database-schema/eav/new", url.Values{
		"name": {"Cliente"}, "machine_name": {"cliente"},
	}, sid)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("cliente entity = %d", rr.Code)
	}
	cliRef := regexp.MustCompile(`/eav/([^/]+)/edit`).FindStringSubmatch(location(t, rr))
	rr = doPostForm(t, mux, "/tools/database-schema/eav/"+cliRef[1]+"/attributes/new", url.Values{
		"machine_name": {"nome"}, "label": {"Nome"}, "primitive_kind": {"TEXT"},
	}, sid)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("nome attribute = %d", rr.Code)
	}

	cliEntity, err := db.Storage.GetEAVEntityTypeByMachineName("cliente")
	if err != nil {
		t.Fatalf("cliente lookup: %v", err)
	}
	anaRec, err := db.Storage.CreateEAVRecord(cliEntity.ID)
	if err != nil {
		t.Fatalf("ana record: %v", err)
	}
	cliAttrs, err := db.Storage.ListEAVAttributesByEntityTypeID(cliEntity.ID)
	if err != nil || len(cliAttrs) != 1 {
		t.Fatalf("cliente attrs: %v", err)
	}
	ana := "Ana Souza"
	err = db.Storage.UpsertEAVValue(anaRec.ID, cliAttrs[0].ID, nil, nil, nil, &ana, nil)
	if err != nil {
		t.Fatalf("ana value: %v", err)
	}
	// Reference choices only offer active records.
	err = db.Storage.UpdateEAVRecordStatus(anaRec.ID, anaRec.Rev, "active")
	if err != nil {
		t.Fatalf("activate ana: %v", err)
	}

	rr = doPostForm(t, mux, "/tools/database-schema/eav/"+etRef[1]+"/attributes/new", url.Values{
		"machine_name": {"cliente"}, "label": {"Cliente"}, "primitive_kind": {"TEXT"},
	}, sid)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("cliente ref attribute = %d", rr.Code)
	}
	chamadoAttrs, err := db.Storage.ListEAVAttributesByEntityTypeID(*form.EAVEntityTypeID)
	if err != nil {
		t.Fatalf("chamado attrs: %v", err)
	}
	var cliAttrRef string
	for _, a := range chamadoAttrs {
		if a.MachineName == "cliente" {
			cliAttrRef = a.ReferenceID
		}
	}
	rr = doPostForm(t, mux, "/tools/forms/"+formRef[1]+"/elements/new", url.Values{
		"machine_name": {"cliente"}, "element_kind": {"field"}, "label": {"Cliente"},
		"eav_attribute_id": {cliAttrRef},
		"ui_kind":          {"reference"},
	}, sid)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("reference element = %d", rr.Code)
	}
	// The plugin options (entity/display) are configured on the element
	// edit screen, exactly like the authoring UI does it.
	formEls, err := db.Storage.ListFormElements(form.ID)
	if err != nil {
		t.Fatalf("ListFormElements: %v", err)
	}
	var refEl *db.FormElement
	for i := range formEls {
		if formEls[i].MachineName == "cliente" {
			refEl = &formEls[i]
		}
	}
	if refEl == nil {
		t.Fatal("reference element not found")
	}
	rr = doPostForm(t, mux,
		"/tools/forms/"+formRef[1]+"/elements/"+refEl.ReferenceID+"/update", url.Values{
			"machine_name": {"cliente"}, "element_kind": {"field"},
			"label": {"Cliente"}, "z_order": {"3"}, "col_span": {"12"},
			"alignment":        {"left"},
			"eav_attribute_id": {cliAttrRef},
			"ui_kind":          {"reference"},
			"ui_meta_json":     {"{\"entity\": \"cliente\", \"display\": \"nome\"}"},
		}, sid)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK {
		t.Fatalf("reference element meta update = %d", rr.Code)
	}

	// The create form offers Ana as a choice; a record pointing at her
	// resolves the display name on the listing.
	rr = doGet(t, mux, "/form/abrir_chamado", sid)
	assertRendered(t, rr, "form with reference")
	if !strings.Contains(rr.Body.String(), "Ana Souza") {
		t.Fatalf("reference choices missing Ana: %.300s", rr.Body.String())
	}
	rr = doPostForm(t, mux, "/form/abrir_chamado", url.Values{
		"assunto": {"Sem internet"}, "prioridade": {"2"},
		"cliente": {anaRec.ReferenceID},
	}, sid)
	if !strings.Contains(location(t, rr), "successfully") {
		t.Fatalf("reference submit rejected: %s", location(t, rr))
	}
	rr = doGet(t, mux, "/form/abrir_chamado/list?q=Ana", sid)
	assertRendered(t, rr, "listing by reference display")
	if !strings.Contains(rr.Body.String(), "Sem internet") {
		t.Fatal("search by reference display missed the record")
	}

	// --- and the wrong password never got in ---
	rr2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(url.Values{"username": {"root"}, "password": {"errada"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(rr2, req)
	if rr2.Code == http.StatusFound {
		t.Fatal("wrong password logged in")
	}
}
