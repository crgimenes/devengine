package handlers

import (
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/devengine/utils"
)

func (h *Handlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	_, _, _, err := auth.Prelude(w, r,
		[]string{
			http.MethodGet,
		},
		false, // check auth
		false, // check ratelimit
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	sid, ok := session.GetCookie(r)
	if ok {
		_, ok := session.Get(sid)
		if ok {
			http.Redirect(w, r, h.cfg.BaseURL+"/", http.StatusFound)
			return
		}
	}

	var u db.User
	authed := false
	u, authed = session.Get(sid)

	data := struct {
		Authed  bool
		User    db.User
		Error   string
		Message string
		Config  config.Config
	}{
		Authed: authed,
		User:   u,
		Config: *h.cfg,
	}

	err = h.templates(w, "login.go.tmpl", data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) LoginMagic(w http.ResponseWriter, r *http.Request) {
	_, _, _, err := auth.Prelude(w, r,
		[]string{
			http.MethodPost,
		},
		false, // check auth
		false, // TODO: check ratelimit to prevent abuse
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	email := r.FormValue("email")
	if email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	if len(email) > 254 {
		http.Error(w, "email too long", http.StatusBadRequest)
		return
	}

	email, err = utils.CanonicalizeEmail(email)
	if err != nil {
		http.Error(w, "invalid email", http.StatusBadRequest)
		return
	}

	token := utils.NewOpaqueID()

	err = db.Storage.StoreMagicLinkToken(token, email, time.Now().UTC().Add(15*time.Minute))
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	link := h.cfg.BaseURL + "/link/" + token

	// No sender configured: "admin generates the link manually" mode. In dev
	// we just redirect to the link; otherwise we log it and tell the user to
	// ask the administrator.
	if h.magicLinkSender == nil {
		if h.cfg.BaseURL == "http://localhost:3210" {
			http.Redirect(w, r, link, http.StatusFound)
			return
		}
		log.Printf("magic link generated for %s (no sender configured): %s", email, link)
		redirectURL := h.cfg.BaseURL +
			"/?message=" +
			url.QueryEscape("Link de acesso gerado. Solicite ao administrador.")
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	err = h.magicLinkSender(email, link)
	if err != nil {
		log.Printf("error delivering magic link to %s: %v", email, err)
		// do not reveal the error to the user
	}

	redirectURL := h.cfg.BaseURL +
		"/?message=" +
		url.QueryEscape("Link de acesso enviado! Verifique seu email.")

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *Handlers) MagicLink(w http.ResponseWriter, r *http.Request) {
	_, _, _, err := auth.Prelude(w, r,
		[]string{
			http.MethodGet,
		},
		false, // check auth
		false, // check ratelimit
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	token := r.PathValue("token")

	if token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	if !utils.ValidateOpaqueID(token) {
		http.Error(w, "invalid token format", http.StatusBadRequest)
		return
	}

	email, err := db.Storage.ConsumeMagicLinkToken(token)
	if err != nil {
		http.Error(w, "invalid or expired token", http.StatusBadRequest)
		return
	}
	if email == "" {
		http.Error(w, "invalid or expired token", http.StatusBadRequest)
		return
	}

	var u *db.User

	u, err = db.Storage.GetUserOrCreateByEmail(email)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	sid := utils.NewOpaqueID()
	session.Put(sid, *u)
	session.SetCookie(w, sid, h.cfg.SessionDuration)

	if u.Username == "" {
		http.Redirect(w, r, h.cfg.BaseURL+"/me", http.StatusFound)
		return
	}

	http.Redirect(w, r, h.cfg.BaseURL+"/", http.StatusFound)
}
