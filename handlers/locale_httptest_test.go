package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/session"
)

// getWithHeader is doGet plus arbitrary headers.
func getWithHeader(t *testing.T, mux *http.ServeMux, path string, c *http.Cookie, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if c != nil {
		req.AddCookie(c)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// Accept-Language must pick the page language when the user has no saved
// preference; without a header the app default (en-US in tests) applies.
func TestRequestLocaleFollowsAcceptLanguage(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)

	rr := getWithHeader(t, mux, "/does-not-exist", nil,
		map[string]string{"Accept-Language": "pt-BR,pt;q=0.9"})
	if !strings.Contains(rr.Body.String(), "Página não encontrada") {
		t.Fatalf("pt-BR header ignored: %.300s", rr.Body.String())
	}

	rr = getWithHeader(t, mux, "/does-not-exist", nil, nil)
	if !strings.Contains(rr.Body.String(), "Page not found") {
		t.Fatalf("default locale not applied: %.300s", rr.Body.String())
	}
}

// A saved user preference beats the browser header.
func TestSavedLocaleBeatsAcceptLanguage(t *testing.T) {
	mux, s := newHTTPTestEnv(t)

	u, err := s.CreateUser("ana", "ana@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	err = s.UpdateUserLocale(u.ID, "pt-BR")
	if err != nil {
		t.Fatalf("UpdateUserLocale: %v", err)
	}
	u, err = s.GetUserByID(u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	sid := "sid-locale-test"
	session.Put(sid, *u)
	cookie := &http.Cookie{Name: "sid", Value: sid}

	rr := getWithHeader(t, mux, "/form/no-such-form", cookie,
		map[string]string{"Accept-Language": "en-US"})
	if !strings.Contains(rr.Body.String(), "Página não encontrada") {
		t.Fatalf("saved pt-BR preference ignored: %.300s", rr.Body.String())
	}
}

// Saving a locale on /me must persist it and refresh the session, so the
// very next page already speaks the chosen language.
func TestProfileSavesLocalePreference(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	user := plantUser(t, "carol", false)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("username", "carol")
	_ = mw.WriteField("email", "carol@example.com")
	_ = mw.WriteField("locale", "pt-BR")
	err := mw.Close()
	if err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/me", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(user)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("POST /me = %d, body: %.300s", rr.Code, rr.Body.String())
	}

	saved, err := db.Storage.GetUserByUsername("carol")
	if err != nil || saved == nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if saved.Locale != "pt-BR" {
		t.Fatalf("locale = %q, want pt-BR", saved.Locale)
	}

	// Session refreshed: an English browser header no longer wins.
	rr = getWithHeader(t, mux, "/form/no-such-form", user,
		map[string]string{"Accept-Language": "en-US"})
	if !strings.Contains(rr.Body.String(), "Página não encontrada") {
		t.Fatalf("session not speaking saved locale: %.300s", rr.Body.String())
	}
}

// The /me page offers the locale select with the registered languages.
func TestProfileShowsLocaleSelect(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	user := plantUser(t, "dave", false)

	rr := doGet(t, mux, "/me", user)
	assertRendered(t, rr, "/me")
	body := rr.Body.String()
	if !strings.Contains(body, `name="locale"`) {
		t.Fatal("locale select missing from /me")
	}
	for _, want := range []string{"en-US", "pt-BR"} {
		if !strings.Contains(body, `value="`+want+`"`) {
			t.Fatalf("locale option %s missing", want)
		}
	}
}

// The default dashboard must list the available forms (translated), so a
// fresh application is usable without a custom home template.
func TestDashboardListsForms(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	user := plantUser(t, "ana", false)
	form := seedTaskForm(t, s, "seed")
	t.Cleanup(func() { i18n.DeleteContent("en-US", form.ReferenceID, "label") })

	rr := doGet(t, mux, "/", user)
	assertRendered(t, rr, "/ (dashboard)")
	body := rr.Body.String()
	if !strings.Contains(body, form.Label) {
		t.Fatalf("dashboard missing form card: %.300s", body)
	}
	if !strings.Contains(body, "/form/"+form.MachineName) {
		t.Fatal("dashboard missing form link")
	}

	// The page chrome follows the request locale (guards the Locale field
	// on the dashboard data struct).
	rr = getWithHeader(t, mux, "/", user, map[string]string{"Accept-Language": "pt-BR"})
	if !strings.Contains(rr.Body.String(), "Formulários") {
		t.Fatal("dashboard chrome not translated to pt-BR")
	}

	// Content translation applies to the card label.
	i18n.SetContent("en-US", form.ReferenceID, "label", "Task entry (en)")
	rr = getWithHeader(t, mux, "/", user, map[string]string{"Accept-Language": "en-US"})
	if !strings.Contains(rr.Body.String(), "Task entry (en)") {
		t.Fatal("dashboard card label not translated")
	}
}
