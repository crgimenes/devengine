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
