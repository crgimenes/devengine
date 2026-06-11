package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/log"
	"github.com/crgimenes/devengine/session"
)

// errorPageData feeds templates/error.go.tmpl. Applications can replace the
// page via SetAppTemplatesFS by shipping their own error.go.tmpl.
type errorPageData struct {
	Authed  bool
	User    db.User
	Config  config.Config
	Status  int
	Title   string
	Message string
	Ref     string
}

// errRef returns a short random id correlating what the user sees with the
// log line carrying the real error.
func errRef() string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	if err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b)
}

// logRef logs err under a new reference id and returns the id, for handlers
// that answer with a redirect message instead of an error page.
func logRef(scope string, err error) string {
	ref := errRef()
	log.Errorw("handler error", "ref", ref, "scope", scope, "err", err)
	return ref
}

// errorTitle returns the page heading for an HTTP status.
func errorTitle(status int) string {
	switch status {
	case http.StatusNotFound:
		return i18n.T("Page not found")
	case http.StatusForbidden:
		return i18n.T("Access denied")
	case http.StatusBadRequest:
		return i18n.T("Bad request")
	default:
		return i18n.T("Internal error")
	}
}

// errorMessage returns the default body text for an HTTP status.
func errorMessage(status int) string {
	switch status {
	case http.StatusNotFound:
		return i18n.T("The address you accessed does not exist or was removed.")
	case http.StatusForbidden:
		return i18n.T("You do not have permission to access this page.")
	case http.StatusBadRequest:
		return i18n.T("The server could not understand the request.")
	default:
		return i18n.T("Something went wrong while processing the request. Try again.")
	}
}

// renderErrorPage writes the standard error page with the given status. The
// session is resolved here so the navbar renders correctly on every path,
// including the ones that fail before authentication.
func (h *Handlers) renderErrorPage(w http.ResponseWriter, r *http.Request, status int, message, ref string) {
	u := db.User{}
	authed := false
	sid, ok := session.GetCookie(r)
	if ok {
		got, ok := session.Get(sid)
		if ok {
			u, authed = got, true
		}
	}

	if message == "" {
		message = errorMessage(status)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	h.render(w, "error.go.tmpl", errorPageData{
		Authed:  authed,
		User:    u,
		Config:  *h.cfg,
		Status:  status,
		Title:   errorTitle(status),
		Message: message,
		Ref:     ref,
	})
}

// errorPage renders the standard error page; an empty message falls back to
// the status default.
func (h *Handlers) errorPage(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.renderErrorPage(w, r, status, message, "")
}

func (h *Handlers) notFound(w http.ResponseWriter, r *http.Request) {
	h.renderErrorPage(w, r, http.StatusNotFound, "", "")
}

func (h *Handlers) forbidden(w http.ResponseWriter, r *http.Request) {
	h.renderErrorPage(w, r, http.StatusForbidden, "", "")
}

// serverError logs the real error under a reference id and shows the generic
// 500 page carrying the same id, so users can report it without the detail
// ever reaching the response.
func (h *Handlers) serverError(w http.ResponseWriter, r *http.Request, scope string, err error) {
	ref := logRef(scope, err)
	h.renderErrorPage(w, r, http.StatusInternalServerError, "", ref)
}
