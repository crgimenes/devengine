package handlers

import (
	"net/http"
	"strings"
	"testing"
)

// Unknown paths fall through the "/" catch-all and must answer a rendered
// 404 page, not the home page with a 200.
func TestUnknownPathRenders404Page(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)

	rr := doGet(t, mux, "/does-not-exist", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /does-not-exist = %d, want 404", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Page not found") {
		t.Fatalf("404 page missing title: %.300s", body)
	}
	if strings.Contains(body, "template error") {
		t.Fatalf("404 page died mid-render: %.300s", body)
	}
}

// The home page itself must keep answering 200 on the exact "/" path.
func TestHomeStillRendersAtRoot(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	assertRendered(t, doGet(t, mux, "/", nil), "/")
}

// A non-sysop hitting a sysop page gets the rendered 403 page.
func TestForbiddenPageForNonSysop(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	user := plantUser(t, "user", false)

	rr := doGet(t, mux, "/tools/forms", user)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("GET /tools/forms as non-sysop = %d, want 403", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Access denied") {
		t.Fatalf("403 page missing title: %.300s", rr.Body.String())
	}
}

// A runtime form that does not exist answers the rendered 404 page.
func TestUnknownFormRenders404Page(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	user := plantUser(t, "user", false)

	rr := doGet(t, mux, "/form/no-such-form", user)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /form/no-such-form = %d, want 404", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Page not found") {
		t.Fatalf("404 page missing title: %.300s", rr.Body.String())
	}
}

// Storage failures must show the generic 500 page with a reference id and
// never the underlying error text.
func TestServerErrorHidesDetailAndShowsRef(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	user := plantUser(t, "user", false)

	// Kill the database under the handler to force a storage error.
	s.Close()

	rr := doGet(t, mux, "/form/whatever", user)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("GET with dead storage = %d, want 500", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Reference code") {
		t.Fatalf("500 page missing reference id: %.300s", body)
	}
	if strings.Contains(body, "database") || strings.Contains(body, "sql") {
		t.Fatalf("500 page leaks the underlying error: %.300s", body)
	}
}
