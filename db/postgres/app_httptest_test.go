package postgres

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	_ "github.com/crgimenes/devengine/eav/ui/defaults"
	"github.com/crgimenes/devengine/handlers"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/devengine/templates"
	"github.com/crgimenes/devengine/utils"
)

// TestAppAgainstPostgres wires the full engine HTTP stack — handlers, real
// templates, plugins — over the PostgreSQL backend and drives an authoring
// plus runtime flow through it. The conformance battery proves the Store
// contract; this proves the whole application path.
func TestAppAgainstPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping container test in -short mode")
	}
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("devengine"),
		tcpostgres.WithUsername("devengine"),
		tcpostgres.WithPassword("devengine"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skipf("could not start postgres container (docker running?): %v", err)
	}
	t.Cleanup(func() {
		err := testcontainers.TerminateContainer(ctr)
		if err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	s, err := NewWithDSN(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(s.Close)
	err = RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}

	prevStorage := db.Storage
	db.Storage = s
	t.Cleanup(func() { db.Storage = prevStorage })

	cfg := &config.Config{
		BaseURL:         "http://localhost:3210",
		LoginURL:        "/login",
		SessionDuration: time.Hour,
		SiteTitle:       "pg-test",
		GitTag:          "test",
	}
	prevCfg := config.Cfg
	config.Cfg = cfg
	t.Cleanup(func() { config.Cfg = prevCfg })

	session.EnableInsecureCookie()
	h := handlers.New(handlers.Dependencies{
		Config:    cfg,
		Templates: templates.ExecuteTemplate,
	})
	mux := http.NewServeMux()
	h.Routes(mux)

	admin, err := s.CreateUser("admin", "admin@example.com", "x", true)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	sid := utils.NewOpaqueID()
	session.Put(sid, *admin)
	t.Cleanup(func() { session.Del(sid) })
	cookie := &http.Cookie{Name: "sid", Value: sid}

	post := func(path string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr
	}
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr
	}

	// Authoring through HTTP: entity, attribute, form, element.
	rr := post("/tools/database-schema/eav/new", url.Values{
		"name": {"Tarefa"}, "machine_name": {"tarefa"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("entity create = %d: %.300s", rr.Code, rr.Body.String())
	}
	et, err := s.GetEAVEntityTypeByMachineName("tarefa")
	if err != nil {
		t.Fatalf("entity lookup: %v", err)
	}
	rr = post("/tools/database-schema/eav/"+et.ReferenceID+"/attributes/new", url.Values{
		"machine_name": {"titulo"}, "label": {"Titulo"}, "primitive_kind": {"TEXT"},
		"is_required": {"1"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("attribute create = %d: %.300s", rr.Code, rr.Body.String())
	}
	rr = post("/tools/forms/new", url.Values{
		"machine_name": {"nova_tarefa"}, "label": {"Nova Tarefa"},
		"entity_type_id": {et.ReferenceID},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("form create = %d: %.300s", rr.Code, rr.Body.String())
	}
	form, err := s.GetFormByMachineName("nova_tarefa")
	if err != nil || form == nil {
		t.Fatalf("form lookup: %v", err)
	}
	attrs, err := s.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil || len(attrs) != 1 {
		t.Fatalf("attrs: %v", err)
	}
	rr = post("/tools/forms/"+form.ReferenceID+"/elements/new", url.Values{
		"machine_name": {"titulo"}, "element_kind": {"field"}, "label": {"Titulo"},
		"eav_attribute_id": {attrs[0].ReferenceID},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("element create = %d: %.300s", rr.Code, rr.Body.String())
	}

	// Runtime: required blocks, valid passes, listing and filter see it.
	rr = post("/form/nova_tarefa", url.Values{"titulo": {""}})
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "required") {
		t.Fatalf("required not enforced: %d %.300s", rr.Code, rr.Body.String())
	}
	rr = post("/form/nova_tarefa", url.Values{"titulo": {"Comprar café"}})
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "successfully") {
		t.Fatalf("valid submit = %d → %s", rr.Code, rr.Header().Get("Location"))
	}
	rr = get("/form/nova_tarefa/list?q=café")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Comprar café") {
		t.Fatalf("listing filter missed record: %d", rr.Code)
	}
}
