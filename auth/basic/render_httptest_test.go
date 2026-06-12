package basic_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/auth/basic"
	"github.com/crgimenes/devengine/templates"
)

// These tests execute the REAL embedded templates. The stub-template harness
// serializes the data struct without touching the templates, so it cannot see
// handler↔template contract bugs (a missing field aborts execution mid-render
// leaving HTTP 200 plus a "template error" tail — exactly how /tools/invites
// shipped broken).

func getPath(h http.Handler, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func assertRendered(t *testing.T, rr *httptest.ResponseRecorder, path string) {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, body: %.300s", path, rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "template error") {
		t.Fatalf("GET %s: template died mid-render", path)
	}
}

func TestLoginPageRendersRealTemplates(t *testing.T) {
	h, cleanup := setupHandlerWith(t, templates.ExecuteTemplate)
	defer cleanup()

	mux := http.NewServeMux()
	h.Routes(mux)

	rr := getPath(mux, "/login")
	assertRendered(t, rr, "/login")
}

func TestSignupPageRendersRealTemplates(t *testing.T) {
	h, cleanup := setupHandlerWith(t, templates.ExecuteTemplate)
	defer cleanup()

	mux := http.NewServeMux()
	h.Routes(mux)

	token, err := basic.CreateInvite("new@example.com")
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	rr := getPath(mux, "/signup/"+token)
	assertRendered(t, rr, "/signup/{token}")
}

// The submit error path re-renders the signup template with an Error message;
// it must carry the same navbar contract as the GET.
func TestSignupSubmitErrorRendersRealTemplates(t *testing.T) {
	h, cleanup := setupHandlerWith(t, templates.ExecuteTemplate)
	defer cleanup()

	mux := http.NewServeMux()
	h.Routes(mux)

	token, err := basic.CreateInvite("new@example.com")
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/signup/"+token,
		strings.NewReader("username=&password="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assertRendered(t, rr, "POST /signup/{token}")
	if !strings.Contains(rr.Body.String(), "Enter a username and password.") {
		t.Fatalf("error message not rendered")
	}
}

// Regression: InvitesPage shipped without CurrentPage in its data struct and
// the sidebar template aborted mid-render on every request.
func TestInvitesPageRendersRealTemplates(t *testing.T) {
	h, cleanup := setupHandlerWith(t, templates.ExecuteTemplate)
	defer cleanup()

	mux := http.NewServeMux()
	h.Routes(mux)

	admin := createSysop(t, "admin", "x")
	sid := plantSession(t, admin)

	rr := getPath(mux, "/tools/invites", sessionCookie(sid))
	assertRendered(t, rr, "/tools/invites")
	if !strings.Contains(rr.Body.String(), "Mint a new invite") {
		t.Fatalf("invites page missing its main form")
	}
}
