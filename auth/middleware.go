// Package auth provides the request authentication entry point (Prelude),
// logout, per-request locale resolution and a pluggable session provider.
package auth

import (
	"net/http"
	"slices"
	"time"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/session"
)

type SessionUser interface {
	ToDBUser() db.User
}

type SessionProvider interface {
	GetCookie(*http.Request) (string, bool)
	SetCookie(http.ResponseWriter, string, time.Duration)
	Get(string) (SessionUser, bool)
	Del(string)
}

var sessions SessionProvider = sessionStore{}

func SetSessionProvider(p SessionProvider) {
	sessions = p
}

type sessionStore struct{}

type storedUser struct {
	db.User
}

func (sessionStore) GetCookie(r *http.Request) (string, bool) { return session.GetCookie(r) }

func (sessionStore) SetCookie(w http.ResponseWriter, value string, maxAge time.Duration) {
	session.SetCookie(w, value, maxAge)
}

func (sessionStore) Get(sid string) (SessionUser, bool) {
	u, ok := session.Get(sid)
	if !ok {
		return nil, false
	}
	return storedUser{User: u}, true
}

func (sessionStore) Del(sid string) { session.Del(sid) }

func (u storedUser) ToDBUser() db.User { return u.User }

// Routes registers the authentication routes on the mux.
func Routes(mux *http.ServeMux) {
	mux.HandleFunc("/logout", Logout)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	sid, ok := sessions.GetCookie(r)
	if ok {
		sessions.Del(sid)
	}
	sessions.SetCookie(w, "", -1) // clear cookie
	http.Redirect(w, r, config.Cfg.BaseURL+"/", http.StatusFound)
}

func Prelude(
	w http.ResponseWriter,
	r *http.Request,
	allowedMethods []string,
	chkAuth bool,
	preventCache bool,
) (
	*db.User,
	string,
	bool,
	error,
) {
	if preventCache {
		h := w.Header()
		h.Set("Cache-Control", "no-store, no-cache, max-age=0, must-revalidate")
		h.Set("Pragma", "no-cache")
		h.Set("Expires", "0")
	}

	if len(allowedMethods) > 0 {
		methodAllowed := slices.Contains(allowedMethods, r.Method)
		if !methodAllowed {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return nil, "", false, nil
		}
	}

	if !chkAuth {
		return nil, "", false, nil
	}

	sid, ok := sessions.GetCookie(r)
	if !ok {
		http.Redirect(w, r, config.Cfg.BaseURL+config.Cfg.LoginURL, http.StatusFound)
		return nil, "", false, nil
	}

	su, ok := sessions.Get(sid)
	if !ok {
		http.Redirect(w, r, config.Cfg.BaseURL+config.Cfg.LoginURL, http.StatusFound)
		return nil, "", false, nil
	}

	u := su.ToDBUser()
	if !u.Enabled {
		sessions.Del(sid)
		sessions.SetCookie(w, "", -1)
		http.Redirect(w, r, config.Cfg.BaseURL+config.Cfg.LoginURL, http.StatusFound)
		return nil, "", false, nil
	}
	return &u, sid, true, nil
}

// RequestLocale resolves the UI language for one request: the session
// user's saved preference, then the browser's Accept-Language, then the
// application default locale (init.filo).
func RequestLocale(r *http.Request) string {
	sid, ok := sessions.GetCookie(r)
	if ok {
		su, ok := sessions.Get(sid)
		if ok {
			u := su.ToDBUser()
			if u.Locale != "" && i18n.Known(u.Locale) {
				return u.Locale
			}
		}
	}
	if loc := i18n.MatchHeader(r.Header.Get("Accept-Language")); loc != "" {
		return loc
	}
	return i18n.Locale()
}
