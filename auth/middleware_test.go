package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/devengine/utils"
)

func init() {
	session.EnableInsecureCookie()
}

// plantSession stores a user session and returns the cookie carrying it.
func plantSession(t *testing.T, u db.User) *http.Cookie {
	t.Helper()
	sid := utils.NewOpaqueID()
	session.Put(sid, u)
	t.Cleanup(func() { session.Del(sid) })
	return &http.Cookie{Name: "sid", Value: sid}
}

func TestPreludeMethodNotAllowed(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)

	user, sid, authed, err := auth.Prelude(rr, req, []string{http.MethodPost}, false, false)
	if err != nil || user != nil || sid != "" || authed {
		t.Fatalf("Prelude = %v, %q, %v, %v", user, sid, authed, err)
	}
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestPreludePreventCacheHeaders(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)

	_, _, _, err := auth.Prelude(rr, req, []string{http.MethodGet}, false, true)
	if err != nil {
		t.Fatalf("Prelude: %v", err)
	}
	cc := rr.Header().Get("Cache-Control")
	if cc == "" || rr.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("cache headers missing: Cache-Control=%q Pragma=%q",
			cc, rr.Header().Get("Pragma"))
	}

	rr = httptest.NewRecorder()
	_, _, _, _ = auth.Prelude(rr, req, []string{http.MethodGet}, false, false)
	if rr.Header().Get("Cache-Control") != "" {
		t.Fatal("cache headers set even with preventCache=false")
	}
}

func TestPreludeRedirectsWithoutSession(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)

	user, _, authed, err := auth.Prelude(rr, req, []string{http.MethodGet}, true, false)
	if err != nil || user != nil || authed {
		t.Fatalf("Prelude = %v, %v, %v", user, authed, err)
	}
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	loc := rr.Header().Get("Location")
	want := config.Cfg.BaseURL + config.Cfg.LoginURL
	if loc != want {
		t.Fatalf("redirect to %q, want %q", loc, want)
	}
}

func TestPreludeRedirectsUnknownSID(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: "sid", Value: "ghost-session"})

	_, _, authed, err := auth.Prelude(rr, req, []string{http.MethodGet}, true, false)
	if err != nil || authed {
		t.Fatalf("Prelude authed=%v, err=%v", authed, err)
	}
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
}

func TestPreludeAuthed(t *testing.T) {
	cookie := plantSession(t, db.User{ID: 42, Username: "ana", Enabled: true})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(cookie)

	user, sid, authed, err := auth.Prelude(rr, req, []string{http.MethodGet}, true, false)
	if err != nil || !authed {
		t.Fatalf("Prelude authed=%v, err=%v", authed, err)
	}
	if user == nil || user.ID != 42 || user.Username != "ana" {
		t.Fatalf("user = %+v", user)
	}
	if sid != cookie.Value {
		t.Fatalf("sid = %q, want %q", sid, cookie.Value)
	}
}

func TestPreludeRejectsDisabledSession(t *testing.T) {
	cookie := plantSession(t, db.User{ID: 47, Username: "disabled"})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(cookie)

	_, _, authed, err := auth.Prelude(rr, req, []string{http.MethodGet}, true, false)
	if err != nil || authed {
		t.Fatalf("disabled session authenticated: authed=%v, err=%v", authed, err)
	}
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	if _, ok := session.Get(cookie.Value); ok {
		t.Fatal("disabled session was not revoked")
	}
}

// An expired session must redirect to login like any anonymous request.
func TestPreludeRejectsExpiredSession(t *testing.T) {
	old := session.MaxSessionAge
	session.MaxSessionAge = -1
	cookie := plantSession(t, db.User{ID: 43, Username: "ghost"})
	session.MaxSessionAge = old

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(cookie)

	_, _, authed, err := auth.Prelude(rr, req, []string{http.MethodGet}, true, false)
	if err != nil || authed {
		t.Fatalf("expired session authenticated: authed=%v, err=%v", authed, err)
	}
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	cookie := plantSession(t, db.User{ID: 44, Username: "out"})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	req.AddCookie(cookie)
	auth.Logout(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	if _, ok := session.Get(cookie.Value); ok {
		t.Fatal("session survives logout")
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("session cookie not cleared: %+v", cookies)
	}
}

func TestRequestLocale(t *testing.T) {
	// A throwaway registered locale exercises the resolver mechanism while the
	// built-in pt-BR dictionary is disabled (see i18n/pt_br.go translationsEnabled).
	i18n.Register("tt-TT", map[string]string{"Page not found": "Pagina nao encontrada (tt)"})

	// Saved preference (known locale) beats the browser header.
	cookie := plantSession(t, db.User{ID: 45, Locale: "tt-TT"})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(cookie)
	req.Header.Set("Accept-Language", "en-US")
	if loc := auth.RequestLocale(req); loc != "tt-TT" {
		t.Fatalf("saved pref ignored: %q", loc)
	}

	// Unknown saved preference falls through to the header.
	cookie = plantSession(t, db.User{ID: 46, Locale: "xx-XX"})
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(cookie)
	req.Header.Set("Accept-Language", "tt-TT,tt;q=0.9")
	if loc := auth.RequestLocale(req); loc != "tt-TT" {
		t.Fatalf("unknown pref did not fall back to header: %q", loc)
	}

	// No session: header wins.
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Accept-Language", "tt-TT")
	if loc := auth.RequestLocale(req); loc != "tt-TT" {
		t.Fatalf("header ignored: %q", loc)
	}

	// Nothing at all: application default.
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	if loc := auth.RequestLocale(req); loc == "" {
		t.Fatal("empty locale resolved")
	}
}
