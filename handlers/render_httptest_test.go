package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	_ "github.com/crgimenes/devengine/eav/ui/defaults"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/devengine/templates"
	"github.com/crgimenes/devengine/utils"
)

// newHTTPTestEnv wires Handlers with the REAL embedded templates over a fresh
// database, routed through a mux so {id} path values resolve. Template errors
// surface mid-render (HTTP 200 + "template error" appended to the partial
// body), so render assertions must inspect the body, not just the status.
func newHTTPTestEnv(t *testing.T) (*http.ServeMux, db.Store) {
	t.Helper()

	s := newValidateTestStore(t)
	t.Cleanup(func() { s.Close() })
	setStorage(t, s)

	cfg := &config.Config{
		BaseURL:         "http://localhost:3210",
		LoginURL:        "/login",
		SessionDuration: time.Hour,
		SiteTitle:       "test",
		GitTag:          "test",
	}
	prevCfg := config.Cfg
	config.Cfg = cfg
	t.Cleanup(func() { config.Cfg = prevCfg })

	session.EnableInsecureCookie()

	h := New(Dependencies{Config: cfg, Templates: templates.ExecuteTemplate})
	mux := http.NewServeMux()
	h.Routes(mux)
	return mux, s
}

func plantUser(t *testing.T, username string, sysop bool) *http.Cookie {
	t.Helper()
	u, err := db.Storage.CreateUser(username, username+"@example.com", "x", sysop)
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", username, err)
	}
	sid := utils.NewOpaqueID()
	session.Put(sid, *u)
	return &http.Cookie{Name: "sid", Value: sid}
}

func doGet(t *testing.T, mux *http.ServeMux, path string, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if c != nil {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func doPostForm(t *testing.T, mux *http.ServeMux, path string, body url.Values, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c != nil {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// assertRendered fails on non-200 or on the "template error" marker appended
// when execution dies mid-render.
func assertRendered(t *testing.T, rr *httptest.ResponseRecorder, path string) {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, body: %.300s", path, rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, "template error") {
		t.Fatalf("GET %s: template died mid-render: %.300s", path, body[max(0, len(body)-300):])
	}
	if len(body) == 0 {
		t.Fatalf("GET %s: empty body", path)
	}
}

// Render smoke over the sysop pages with the real templates. Catches the
// handler↔template contract class (missing data struct field aborts the
// template mid-render), which stub-template harnesses cannot see.
func TestSysopPagesRenderWithRealTemplates(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	paths := []string{
		"/tools",
		"/tools/database-schema",
		"/tools/forms",
		"/tools/users",
		"/tools/menu-editor",
		"/tools/filo",
		"/tools/search-forms",
		"/me",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			assertRendered(t, doGet(t, mux, p, admin), p)
		})
	}
}

func TestToolsForbiddenForNonSysop(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	user := plantUser(t, "user", false)

	rr := doGet(t, mux, "/tools", user)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("GET /tools as non-sysop = %d, want 403", rr.Code)
	}
}

// seedTaskForm creates an entity type with one TEXT attribute, a form bound to
// it, and one record holding the given title.
func seedTaskForm(t *testing.T, s db.Store, title string) *db.Form {
	t.Helper()
	et, err := s.CreateEAVEntityType("Tarefa", "tarefa", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	attr := textAttr(t, s, et.ID, "titulo", "Título")
	form, err := s.CreateForm("cadastro", "Cadastro", "", &et.ID)
	if err != nil {
		t.Fatalf("CreateForm: %v", err)
	}
	_, err = s.CreateFormElement(form.ID, nil, "titulo", "field", "Título", "", 0, 12, "", "", &attr.ID, false, false)
	if err != nil {
		t.Fatalf("CreateFormElement: %v", err)
	}
	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord: %v", err)
	}
	err = s.UpsertEAVValue(rec.ID, attr.ID, nil, nil, nil, &title, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue: %v", err)
	}
	return form
}

// The end-user listing must work for a plain authenticated (non-sysop) user:
// full page, HTMX rows fragment, text filter, and the create view.
func TestFormsRuntimeListEndUser(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	seedTaskForm(t, s, "Comprar leite")
	user := plantUser(t, "user", false)

	rr := doGet(t, mux, "/form/cadastro/list", user)
	assertRendered(t, rr, "/form/cadastro/list")
	if !strings.Contains(rr.Body.String(), "Comprar leite") {
		t.Fatalf("listing does not show the record")
	}

	rr = doGet(t, mux, "/form/cadastro/list/rows", user)
	assertRendered(t, rr, "/form/cadastro/list/rows")
	if !strings.Contains(rr.Body.String(), "Comprar leite") {
		t.Fatalf("rows fragment does not show the record")
	}

	rr = doGet(t, mux, "/form/cadastro/list?q=leite", user)
	if !strings.Contains(rr.Body.String(), "Comprar leite") {
		t.Fatalf("filter match should keep the record visible")
	}
	rr = doGet(t, mux, "/form/cadastro/list?q=zzz", user)
	if strings.Contains(rr.Body.String(), "Comprar leite") {
		t.Fatalf("filter miss should hide the record")
	}

	rr = doGet(t, mux, "/form/cadastro", user)
	assertRendered(t, rr, "/form/cadastro")
}

func TestFormsRuntimeListRequiresAuth(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	seedTaskForm(t, s, "secreta")

	rr := doGet(t, mux, "/form/cadastro/list", nil)
	if rr.Code < 300 || rr.Code > 399 {
		t.Fatalf("unauthenticated GET /form/cadastro/list = %d, want redirect", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "secreta") {
		t.Fatalf("unauthenticated response leaked record data")
	}
}

// The sysop records viewer rows fragment had a define-only template executed
// by filename, which renders nothing. Lock the fix.
func TestToolsFormsRecordsRowsFragment(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form := seedTaskForm(t, s, "Comprar leite")
	admin := plantUser(t, "admin", true)

	path := "/tools/forms/" + form.ReferenceID + "/records/rows"
	rr := doGet(t, mux, path, admin)
	assertRendered(t, rr, path)
	if !strings.Contains(rr.Body.String(), "Comprar leite") {
		t.Fatalf("sysop rows fragment does not show the record")
	}
}

// Covers the internal path of the form editor's "Salvar Alterações": binding a
// menu to a form must persist menu_id, and clearing the select must unbind.
func TestToolsFormsUpdatePersistsMenuBinding(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	menu, err := s.CreateMenu("tarefas", "Tarefas", "")
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	form, err := s.CreateForm("cadastro", "Cadastro", "", nil)
	if err != nil {
		t.Fatalf("CreateForm: %v", err)
	}

	body := url.Values{
		"machine_name": {"cadastro"},
		"label":        {"Cadastro"},
		"menu_id":      {menu.ReferenceID},
	}
	rr := doPostForm(t, mux, "/tools/forms/"+form.ReferenceID+"/update", body, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("update = %d, want 303", rr.Code)
	}
	if loc := rr.Header().Get("Location"); strings.Contains(loc, "Erro") {
		t.Fatalf("update redirected with error: %s", loc)
	}

	got, err := s.GetFormByRefID(form.ReferenceID)
	if err != nil {
		t.Fatalf("GetFormByRefID: %v", err)
	}
	if got.MenuID == nil || *got.MenuID != menu.ID {
		t.Fatalf("menu_id not persisted: %v", got.MenuID)
	}

	body.Set("menu_id", "")
	rr = doPostForm(t, mux, "/tools/forms/"+form.ReferenceID+"/update", body, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("unbind update = %d, want 303", rr.Code)
	}
	got, err = s.GetFormByRefID(form.ReferenceID)
	if err != nil {
		t.Fatalf("GetFormByRefID: %v", err)
	}
	if got.MenuID != nil {
		t.Fatalf("menu_id should be cleared, got %d", *got.MenuID)
	}
}

// Render smoke over the forms-authoring detail pages (create, edit, element
// edit, records) — the i18n template sweep must not kill any of them.
func TestFormsAuthoringPagesRenderWithRealTemplates(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	form := seedTaskForm(t, s, "seed")

	els, err := s.ListFormElements(form.ID)
	if err != nil || len(els) == 0 {
		t.Fatalf("ListFormElements: %v", err)
	}

	paths := []string{
		"/tools/forms/new",
		"/tools/forms/" + form.ReferenceID + "/edit",
		"/tools/forms/" + form.ReferenceID + "/elements/" + els[0].ReferenceID + "/edit",
		"/tools/forms/" + form.ReferenceID + "/records",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			assertRendered(t, doGet(t, mux, p, admin), p)
		})
	}
}

// Render smoke over the database-schema authoring pages, ahead of the i18n
// template sweep (slice 2).
func TestSchemaAuthoringPagesRenderWithRealTemplates(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	ent := seedAdminEntity(t, s, "")

	attrs, err := s.ListEAVAttributesByEntityTypeID(ent.et.ID)
	if err != nil || len(attrs) == 0 {
		t.Fatalf("attributes: %v", err)
	}
	rec, err := s.CreateEAVRecord(ent.et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord: %v", err)
	}

	base := "/tools/database-schema/eav/" + ent.et.ReferenceID
	paths := []string{
		"/tools/database-schema",
		"/tools/database-schema/eav/new",
		base + "/edit",
		base + "/attributes/new",
		base + "/attributes/" + attrs[0].ReferenceID + "/edit",
		base + "/records",
		base + "/records/new",
		base + "/records/" + rec.ReferenceID + "/edit",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			assertRendered(t, doGet(t, mux, p, admin), p)
		})
	}
}

// Render smoke over menu-editor and users detail pages (i18n slice 3).
func TestMenuAndUserPagesRenderWithRealTemplates(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	menu, err := s.CreateMenu("m1", "Menu Um", "")
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}
	item, err := s.CreateMenuItem(menu.ID, nil, "home", "Home", "", "link", "/", "", "", 0)
	if err != nil {
		t.Fatalf("CreateMenuItem: %v", err)
	}
	target := plantUserRecord(t, "bob")

	paths := []string{
		"/tools/menu-editor/new",
		"/tools/menu-editor/" + menu.ReferenceID + "/edit",
		"/tools/menu-editor/" + menu.ReferenceID + "/items/" + item.ReferenceID + "/edit",
		"/tools/users/" + target + "/edit",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			assertRendered(t, doGet(t, mux, p, admin), p)
		})
	}
}

// plantUserRecord creates a plain user and returns its reference id.
func plantUserRecord(t *testing.T, username string) string {
	t.Helper()
	u, err := db.Storage.CreateUser(username, username+"@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", username, err)
	}
	return u.ReferenceID
}
