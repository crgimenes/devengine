package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/i18n"
)

func TestTranslationsScreenListsKeys(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	// Built-in pt-BR is disabled (see i18n/pt_br.go translationsEnabled), so a
	// throwaway registered locale supplies the key/translation the editor lists.
	i18n.Register("tt-TT", map[string]string{"Page not found": "Pagina nao encontrada (tt)"})

	rr := doGet(t, mux, "/tools/translations?locale=tt-TT", admin)
	assertRendered(t, rr, "/tools/translations")
	body := rr.Body.String()
	if !strings.Contains(body, "Page not found") {
		t.Fatal("known key missing from the editor")
	}
	if !strings.Contains(body, "Pagina nao encontrada (tt)") {
		t.Fatal("effective translation missing from the editor")
	}
}

func TestTranslationsSaveAppliesLiveAndRestores(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	t.Cleanup(func() { i18n.DeleteOverride("pt-BR", "Page not found") })

	rr := doPostForm(t, mux, "/tools/translations", url.Values{
		"locale":      {"pt-BR"},
		"msg_key":     {"Page not found"},
		"translation": {"Página sumiu!"},
	}, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("save = %d: %.300s", rr.Code, rr.Body.String())
	}

	// Applied live: the 404 page speaks the adjusted text.
	rr = getWithHeader(t, mux, "/does-not-exist", nil,
		map[string]string{"Accept-Language": "pt-BR"})
	if !strings.Contains(rr.Body.String(), "Página sumiu!") {
		t.Fatalf("override not applied live: %.300s", rr.Body.String())
	}

	// Persisted for the next boot.
	overrides, err := s.ListI18nOverrides()
	if err != nil || len(overrides) != 1 {
		t.Fatalf("overrides = %v (%v)", overrides, err)
	}
	if overrides[0].Translation != "Página sumiu!" {
		t.Fatalf("stored translation = %q", overrides[0].Translation)
	}

	// Empty text clears the override row. With the built-in pt-BR dictionary
	// disabled (see i18n/pt_br.go translationsEnabled), pt-BR is no longer a
	// known locale, so the page falls back to the English key.
	rr = doPostForm(t, mux, "/tools/translations", url.Values{
		"locale":      {"pt-BR"},
		"msg_key":     {"Page not found"},
		"translation": {""},
	}, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("restore = %d", rr.Code)
	}
	rr = getWithHeader(t, mux, "/does-not-exist", nil,
		map[string]string{"Accept-Language": "pt-BR"})
	if !strings.Contains(rr.Body.String(), "Page not found") {
		t.Fatalf("English key not restored after clearing override: %.300s", rr.Body.String())
	}
	overrides, err = s.ListI18nOverrides()
	if err != nil || len(overrides) != 0 {
		t.Fatalf("overrides after restore = %v (%v)", overrides, err)
	}
}

// Adding a key for a brand-new locale makes that locale selectable and
// served to matching browsers.
func TestTranslationsNewLocale(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	t.Cleanup(func() { i18n.DeleteOverride("es-ES", "Page not found") })

	rr := doPostForm(t, mux, "/tools/translations", url.Values{
		"locale":      {"es-ES"},
		"msg_key":     {"Page not found"},
		"translation": {"Página no encontrada"},
	}, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("save = %d", rr.Code)
	}

	if !i18n.Known("es-ES") {
		t.Fatal("new locale not known after override")
	}
	rr = getWithHeader(t, mux, "/does-not-exist", nil,
		map[string]string{"Accept-Language": "es-ES,es;q=0.9"})
	if !strings.Contains(rr.Body.String(), "Página no encontrada") {
		t.Fatalf("new locale not served: %.300s", rr.Body.String())
	}
}

// Persisted overrides must be layered back at boot (handlers.New).
func TestTranslationOverridesLoadAtBoot(t *testing.T) {
	_, s := newHTTPTestEnv(t)
	t.Cleanup(func() { i18n.DeleteOverride("pt-BR", "Access denied") })

	err := s.UpsertI18nOverride("pt-BR", "Access denied", "Sem entrada!")
	if err != nil {
		t.Fatalf("UpsertI18nOverride: %v", err)
	}
	loadTranslationOverrides()

	got := i18n.TL("pt-BR", "Access denied")
	if got != "Sem entrada!" {
		t.Fatalf("TL after boot load = %q", got)
	}
}

func TestTranslationsForbiddenForNonSysop(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	user := plantUser(t, "user", false)

	rr := doGet(t, mux, "/tools/translations", user)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-sysop = %d, want 403", rr.Code)
	}
}

// The default locale is read-only: keys ARE the English text.
func TestTranslationsRejectDefaultLocaleEdit(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	rr := doPostForm(t, mux, "/tools/translations", url.Values{
		"locale":      {"en-US"},
		"msg_key":     {"Page not found"},
		"translation": {"hacked"},
	}, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("save = %d", rr.Code)
	}
	overrides, err := s.ListI18nOverrides()
	if err != nil || len(overrides) != 0 {
		t.Fatalf("default locale override stored: %v (%v)", overrides, err)
	}
}

// Content translations must show on runtime screens (form title, element
// label) per request locale, while authoring screens keep the originals.
func TestContentTranslationOnRuntime(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	user := plantUser(t, "user", false)
	form := seedTaskForm(t, s, "seed")

	els, err := s.ListFormElements(form.ID)
	if err != nil || len(els) == 0 {
		t.Fatalf("ListFormElements: %v", err)
	}
	el := els[0]
	t.Cleanup(func() {
		i18n.DeleteContent("en-US", form.ReferenceID, "label")
		i18n.DeleteContent("en-US", el.ReferenceID, "label")
	})

	for _, c := range []struct{ ref, field, text string }{
		{form.ReferenceID, "label", "Task entry"},
		{el.ReferenceID, "label", "Title (en)"},
	} {
		rr := doPostForm(t, mux, "/tools/translations/content", url.Values{
			"locale": {"en-US"}, "ref_id": {c.ref}, "field": {c.field}, "text": {c.text},
		}, admin)
		if rr.Code != http.StatusSeeOther {
			t.Fatalf("content save = %d: %.200s", rr.Code, rr.Body.String())
		}
	}

	// English browser sees the translations on the runtime form.
	rr := getWithHeader(t, mux, "/form/"+form.MachineName, user,
		map[string]string{"Accept-Language": "en-US"})
	assertRendered(t, rr, "/form (en)")
	body := rr.Body.String()
	if !strings.Contains(body, "Task entry") || !strings.Contains(body, "Title (en)") {
		t.Fatalf("content translation missing on runtime: %.300s", body)
	}

	// Portuguese browser keeps the original content.
	rr = getWithHeader(t, mux, "/form/"+form.MachineName, user,
		map[string]string{"Accept-Language": "pt-BR"})
	if strings.Contains(rr.Body.String(), "Task entry") {
		t.Fatal("en translation leaked into pt-BR request")
	}

	// Authoring keeps the original even for an English browser.
	rr = getWithHeader(t, mux,
		"/tools/forms/"+form.ReferenceID+"/elements/"+el.ReferenceID+"/edit",
		admin, map[string]string{"Accept-Language": "en-US"})
	assertRendered(t, rr, "/element edit")
	if strings.Contains(rr.Body.String(), "Title (en)") {
		t.Fatal("authoring screen showed the translated label")
	}

	// Empty text restores the original on the next render.
	rr = doPostForm(t, mux, "/tools/translations/content", url.Values{
		"locale": {"en-US"}, "ref_id": {form.ReferenceID}, "field": {"label"}, "text": {""},
	}, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("restore = %d", rr.Code)
	}
	rr = getWithHeader(t, mux, "/form/"+form.MachineName, user,
		map[string]string{"Accept-Language": "en-US"})
	if strings.Contains(rr.Body.String(), "Task entry") {
		t.Fatal("original form label not restored")
	}
}

// The Go export must be a compilable-shaped dictionary with the overrides.
func TestTranslationsExportGo(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	t.Cleanup(func() { i18n.DeleteOverride("pt-BR", "Page not found") })

	rr := doPostForm(t, mux, "/tools/translations", url.Values{
		"locale": {"pt-BR"}, "msg_key": {"Page not found"}, "translation": {"Cadê a página?"},
	}, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("save = %d", rr.Code)
	}

	rr = doGet(t, mux, "/tools/translations/export.go?locale=pt-BR&mode=overrides", admin)
	if rr.Code != http.StatusOK {
		t.Fatalf("export.go = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"package i18n", `Register("pt-BR"`, `"Page not found": "Cadê a página?",`} {
		if !strings.Contains(body, want) {
			t.Fatalf("%q missing from Go export: %.300s", want, body)
		}
	}

	// merged mode carries every effective translation. With the built-in
	// pt-BR dictionary disabled (see i18n/pt_br.go translationsEnabled), the
	// override is the only effective entry, so it must still appear here.
	rr = doGet(t, mux, "/tools/translations/export.go?locale=pt-BR", admin)
	if !strings.Contains(rr.Body.String(), `"Page not found": "Cadê a página?",`) {
		t.Fatalf("merged export missing effective entry")
	}
}

// CSV import applies and persists each row as an override.
func TestTranslationsImportCSV(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)
	t.Cleanup(func() {
		i18n.DeleteOverride("pt-BR", "Page not found")
		i18n.DeleteOverride("pt-BR", "Access denied")
	})

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("locale", "pt-BR")
	fw, err := mw.CreateFormFile("file", "t.csv")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write([]byte("key,translation\nPage not found,Sumiu\nAccess denied,Barrado\n"))
	err = mw.Close()
	if err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/tools/translations/import", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("import = %d: %.300s", rr.Code, rr.Body.String())
	}

	if i18n.TL("pt-BR", "Page not found") != "Sumiu" {
		t.Fatal("imported translation not applied live")
	}
	overrides, err := s.ListI18nOverrides()
	if err != nil || len(overrides) != 2 {
		t.Fatalf("overrides = %v (%v)", overrides, err)
	}
}

// A pre_save block authored in Filo is a user message and passes verbatim;
// an infrastructure failure (broken script) shows the generic text + ref id
// and never the internal detail. The db.UserError sentinel drives the split.
func TestSaveErrorSentinelSeparatesUserFromInfra(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	user := plantUser(t, "user", false)

	seed := func(machine, preSave string) {
		t.Helper()
		et, err := s.CreateEAVEntityType(machine, machine, "", preSave, "")
		if err != nil {
			t.Fatalf("CreateEAVEntityType(%s): %v", machine, err)
		}
		attr, err := s.CreateEAVAttribute(et.ID, "nome", "Nome", "", "TEXT",
			false, false, false, nil, false, "", nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateEAVAttribute: %v", err)
		}
		form, err := s.CreateForm(machine, machine, "", &et.ID)
		if err != nil {
			t.Fatalf("CreateForm: %v", err)
		}
		_, err = s.CreateFormElement(form.ID, nil, "nome", "field", "Nome",
			"", 0, 12, "", "", &attr.ID, false, false)
		if err != nil {
			t.Fatalf("CreateFormElement: %v", err)
		}
	}

	seed("blocked", `(set error "bloqueado pelo script")`)
	seed("broken", `(((this is not filo`)

	rr := doPostForm(t, mux, "/form/blocked", url.Values{"nome": {"x"}}, user)
	if rr.Code != http.StatusOK {
		t.Fatalf("blocked submit = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "bloqueado pelo script") {
		t.Fatalf("script message did not pass through: %.300s", rr.Body.String())
	}

	rr = doPostForm(t, mux, "/form/broken", url.Values{"nome": {"x"}}, user)
	if rr.Code != http.StatusOK {
		t.Fatalf("broken submit = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Could not save (ref ") {
		t.Fatalf("generic infra message missing: %.300s", body)
	}
	if strings.Contains(body, "pre_save script") || strings.Contains(body, "parse") {
		t.Fatalf("infra detail leaked to the page: %.300s", body)
	}
}
