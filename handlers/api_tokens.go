package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/utils"
)

// meData is the page model for /me. NewToken carries the plaintext of a
// token minted in this very response — it is never persisted or put in a
// URL, so it is visible exactly once.
type meData struct {
	Authed   bool
	User     db.User
	Error    string
	Message  string
	Config   config.Config
	Locale   string
	Locales  []string
	Tokens   []db.APIToken
	NewToken string
}

// renderMe renders the profile page with the user's API tokens loaded.
func (h *Handlers) renderMe(w http.ResponseWriter, r *http.Request, u *db.User, data meData) {
	tokens, err := db.Storage.ListAPITokensByUserID(u.ID)
	if err != nil {
		h.serverError(w, r, "ListAPITokensByUserID", err)
		return
	}
	data.Authed = true
	data.User = *u
	data.Config = *h.cfg
	data.Locale = auth.RequestLocale(r)
	data.Locales = i18n.Locales()
	data.Tokens = tokens
	h.render(w, "me.go.tmpl", data)
}

// APITokenCreate handles POST /me/api-tokens: mints a bearer token, stores
// only its hash and shows the plaintext once on the rendered page.
func (h *Handlers) APITokenCreate(w http.ResponseWriter, r *http.Request) {
	u, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "APITokenCreate", err)
		return
	}
	if !authed {
		return
	}

	label := strings.TrimSpace(r.FormValue("label"))
	if len(label) > 80 {
		label = label[:80]
	}

	token := utils.NewOpaqueID()
	_, err = db.Storage.CreateAPIToken(u.ID, hashAPIToken(token), label)
	if err != nil {
		ref := logRef("CreateAPIToken", err)
		h.renderMe(w, r, u, meData{Error: tr(r, "Could not create the token (ref %s)", ref)})
		return
	}

	h.renderMe(w, r, u, meData{
		Message:  tr(r, "Token created."),
		NewToken: token,
	})
}

// APITokenDelete handles POST /me/api-tokens/{id}/delete (revocation).
func (h *Handlers) APITokenDelete(w http.ResponseWriter, r *http.Request) {
	u, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "APITokenDelete", err)
		return
	}
	if !authed {
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.errorPage(w, r, http.StatusBadRequest, "invalid token id")
		return
	}
	err = db.Storage.DeleteAPIToken(id, u.ID)
	if err != nil {
		ref := logRef("DeleteAPIToken", err)
		http.Redirect(w, r, "/me?error="+url.QueryEscape(tr(r, "Could not revoke the token (ref %s)", ref)), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/me?message="+url.QueryEscape(tr(r, "Token revoked.")), http.StatusSeeOther)
}
