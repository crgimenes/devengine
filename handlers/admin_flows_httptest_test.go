package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// doPostCSRF posts a form with the CSRF double-submit pair attached.
func doPostCSRF(t *testing.T, mux *http.ServeMux, path string, body url.Values, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	body.Set("csrf_token", "test-csrf-token")
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c != nil {
		req.AddCookie(c)
	}
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "test-csrf-token"})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// TestMenusAdminFlow drives the menu editor end to end: menu CRUD, items
// with submenu nesting, reordering and soft delete.
func TestMenusAdminFlow(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	// Create menu.
	rr := doPostCSRF(t, mux, "/tools/menu-editor/new", url.Values{
		"machine_name": {"principal"},
		"label":        {"Principal"},
		"description":  {"menu de teste"},
	}, admin)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
		t.Fatalf("menu create = %d: %.300s", rr.Code, rr.Body.String())
	}
	menu, err := s.GetMenuByMachineName("principal")
	if err != nil || menu == nil {
		t.Fatalf("menu not created: %v", err)
	}

	// Update menu.
	rr = doPostCSRF(t, mux, "/tools/menu-editor/"+menu.ReferenceID+"/update", url.Values{
		"machine_name": {"principal"},
		"label":        {"Menu Principal"},
		"description":  {"atualizado"},
	}, admin)
	if rr.Code >= 400 {
		t.Fatalf("menu update = %d", rr.Code)
	}
	menu, _ = s.GetMenuByMachineName("principal")
	if menu.Label != "Menu Principal" {
		t.Fatalf("update not persisted: %+v", menu)
	}

	// Create two items; creation lands on the item editor.
	rr = doPostCSRF(t, mux, "/tools/menu-editor/"+menu.ReferenceID+"/items/new", url.Values{
		"machine_name": {"home"},
		"label":        {"Home"},
		"icon":         {"bi-house"},
	}, admin)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
		t.Fatalf("item create = %d: %.300s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "/items/") || !strings.Contains(loc, "/edit") {
		t.Fatalf("item create must land on the item editor, got %q", loc)
	}
	rr = doPostCSRF(t, mux, "/tools/menu-editor/"+menu.ReferenceID+"/items/new", url.Values{
		"machine_name": {"sobre"},
		"label":        {"Sobre"},
	}, admin)
	if rr.Code >= 400 {
		t.Fatalf("item 2 create = %d", rr.Code)
	}

	items, err := s.ListMenuItems(menu.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("ListMenuItems = %d, %v", len(items), err)
	}
	home, sobre := items[0], items[1]
	if home.MachineName != "home" {
		home, sobre = sobre, home
	}

	// Update an item to a link with URL.
	rr = doPostCSRF(t, mux, "/tools/menu-editor/"+menu.ReferenceID+"/items/"+sobre.ReferenceID+"/update", url.Values{
		"machine_name": {"sobre"},
		"label":        {"Sobre nós"},
		"item_type":    {"link"},
		"url":          {"/sobre"},
	}, admin)
	if rr.Code >= 400 {
		t.Fatalf("item update = %d: %.300s", rr.Code, rr.Body.String())
	}
	got, _ := s.GetMenuItemByRefID(sobre.ReferenceID)
	if got.Label != "Sobre nós" || got.URL != "/sobre" {
		t.Fatalf("item update not persisted: %+v", got)
	}

	// Reorder: move the second item up, then verify the order flipped.
	rr = doPostCSRF(t, mux, "/tools/menu-editor/"+menu.ReferenceID+"/items/"+sobre.ReferenceID+"/up", url.Values{}, admin)
	if rr.Code >= 400 {
		t.Fatalf("move up = %d", rr.Code)
	}
	items, _ = s.ListMenuItems(menu.ID)
	if items[0].MachineName != "sobre" {
		t.Fatalf("move up did not reorder: first is %q", items[0].MachineName)
	}

	// Preview renders.
	rr = doGet(t, mux, "/tools/menu-editor/"+menu.ReferenceID+"/preview", admin)
	assertRendered(t, rr, "menu preview")

	// Delete item, then soft delete the menu.
	rr = doPostCSRF(t, mux, "/tools/menu-editor/"+menu.ReferenceID+"/items/"+home.ReferenceID+"/delete", url.Values{}, admin)
	if rr.Code >= 400 {
		t.Fatalf("item delete = %d", rr.Code)
	}
	items, _ = s.ListMenuItems(menu.ID)
	if len(items) != 1 {
		t.Fatalf("item not deleted: %d left", len(items))
	}
	rr = doPostCSRF(t, mux, "/tools/menu-editor/"+menu.ReferenceID+"/delete", url.Values{}, admin)
	if rr.Code >= 400 {
		t.Fatalf("menu delete = %d", rr.Code)
	}
	menus, _ := s.ListMenus()
	if len(menus) != 0 {
		t.Fatalf("menu not soft-deleted: %d listed", len(menus))
	}
}

// Menu mutations require the CSRF double-submit pair.
func TestMenusRejectMissingCSRF(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	rr := doPostForm(t, mux, "/tools/menu-editor/new", url.Values{
		"machine_name": {"semcsrf"},
		"label":        {"X"},
	}, admin)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF = %d, want 403", rr.Code)
	}
}

// TestUsersAdminFlow drives the sysop users manager: listing, edit form,
// profile/flags update and password reset.
func TestUsersAdminFlow(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	admin := plantUser(t, "admin", true)

	target, err := s.CreateUser("carlos", "carlos@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Listing shows the user.
	rr := doGet(t, mux, "/tools/users", admin)
	assertRendered(t, rr, "/tools/users")
	if !strings.Contains(rr.Body.String(), "carlos") {
		t.Fatal("listing missing user")
	}

	// Edit form renders.
	rr = doGet(t, mux, "/tools/users/"+target.ReferenceID+"/edit", admin)
	assertRendered(t, rr, "user edit")

	// Update profile + flags.
	rr = doPostForm(t, mux, "/tools/users/"+target.ReferenceID+"/update", url.Values{
		"username": {"carlos2"},
		"email":    {"carlos2@example.com"},
		"sysop":    {"1"},
		"enabled":  {"1"},
	}, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("user update = %d: %.300s", rr.Code, rr.Body.String())
	}
	updated, _ := s.GetUserByID(target.ID)
	if updated.Username != "carlos2" || !updated.Sysop || !updated.Enabled {
		t.Fatalf("update not persisted: %+v", updated)
	}

	// Password reset surfaces a fresh secret once via redirect.
	rr = doPostForm(t, mux, "/tools/users/"+target.ReferenceID+"/reset-password", url.Values{}, admin)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("reset = %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "new_password=") {
		t.Fatalf("reset redirect missing password: %q", loc)
	}
	after, _ := s.GetUserByID(target.ID)
	if after.PasswordHash == "x" || after.PasswordHash == "" {
		t.Fatal("password hash not rotated")
	}

	// Unknown ref is a 404.
	rr = doGet(t, mux, "/tools/users/nope/edit", admin)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown user = %d, want 404", rr.Code)
	}
}

// The users manager is sysop-only.
func TestUsersForbiddenForNonSysop(t *testing.T) {
	mux, _ := newHTTPTestEnv(t)
	user := plantUser(t, "comum", false)

	rr := doGet(t, mux, "/tools/users", user)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-sysop listing = %d, want 403", rr.Code)
	}

	rr = doPostForm(t, mux, "/tools/users/whatever/update", url.Values{
		"username": {"hack"},
	}, user)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("non-sysop update = %d, want 403", rr.Code)
	}
}
