// Package basic implements username/password authentication with
// invite-based signup: login, signup and the sysop invites panel.
package basic

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/crgimenes/devengine/log"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/ratelimit"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/devengine/utils"
)

// TemplateExecutor renders a named template into w using data. Matches the
// shape of devengine/handlers.TemplateExecutor and templates.ExecuteTemplate
// without creating a hard dependency on the handlers package.
type TemplateExecutor func(w io.Writer, name string, data any) error

// Handlers wires the basic auth HTTP handlers with the configuration and the
// template executor they need.
type Handlers struct {
	cfg       *config.Config
	templates TemplateExecutor
}

// New returns a configured *Handlers. cfg defaults to config.Cfg when nil.
func New(cfg *config.Config, tmpl TemplateExecutor) *Handlers {
	if cfg == nil {
		cfg = config.Cfg
	}
	return &Handlers{cfg: cfg, templates: tmpl}
}

// tr translates msg into the locale resolved for this request.
func tr(r *http.Request, msg string, args ...any) string {
	return i18n.TL(auth.RequestLocale(r), msg, args...)
}

// rateLimited reports whether this client exhausted the auth-endpoint budget
// and stamps Retry-After when it did. Limits come from the instance config
// (init.filo via the application), RateLimitPerMin <= 0 disables the check.
func (h *Handlers) rateLimited(w http.ResponseWriter, r *http.Request) bool {
	ip := ratelimit.ClientIP(r, h.cfg.RateLimitTrustProxy)
	if ratelimit.Default.Allow(ip, h.cfg.RateLimitPerMin, h.cfg.RateLimitBurst) {
		return false
	}
	w.Header().Set("Retry-After", "60")
	return true
}

// LoginPage renders the login form.
func (h *Handlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	_, _, _, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		false, // check auth
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

	data := struct {
		Authed  bool
		User    db.User
		Error   string
		Message string
		Config  config.Config
		Locale  string
	}{
		Authed: false,
		Config: *h.cfg,
		Locale: auth.RequestLocale(r),
	}

	err = h.templates(w, "login.go.tmpl", data)
	if err != nil {
		log.Printf("template error in login.go.tmpl: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// LoginSubmit validates username + password and starts a session on success.
func (h *Handlers) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	_, _, _, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		false, // check auth
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if h.rateLimited(w, r) {
		w.WriteHeader(http.StatusTooManyRequests)
		h.renderLoginError(w, r, tr(r, "Too many attempts. Wait a moment and try again."))
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if username == "" || password == "" {
		h.renderLoginError(w, r, tr(r, "Enter username and password."))
		return
	}

	u, err := db.Storage.GetUserByUsername(username)
	if err != nil {
		log.Printf("login: GetUserByUsername(%q): %v", username, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if u == nil || u.PasswordHash == "" || !u.Enabled {
		h.renderLoginError(w, r, tr(r, "Invalid credentials."))
		return
	}

	if !VerifyPassword(u.PasswordHash, password) {
		h.renderLoginError(w, r, tr(r, "Invalid credentials."))
		return
	}

	sid := utils.NewOpaqueID()
	session.Put(sid, *u)
	session.SetCookie(w, sid, h.cfg.SessionDuration)

	http.Redirect(w, r, h.cfg.BaseURL+"/", http.StatusFound)
}

func (h *Handlers) renderLoginError(w http.ResponseWriter, r *http.Request, message string) {
	data := struct {
		Authed  bool
		User    db.User
		Error   string
		Message string
		Config  config.Config
		Locale  string
	}{
		Error:  message,
		Config: *h.cfg,
		Locale: auth.RequestLocale(r),
	}
	err := h.templates(w, "login.go.tmpl", data)
	if err != nil {
		log.Printf("template error in login.go.tmpl: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// SignupPage renders the invite acceptance form when the token is valid.
func (h *Handlers) SignupPage(w http.ResponseWriter, r *http.Request) {
	_, _, _, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		false, // check auth
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if h.rateLimited(w, r) {
		http.Error(w, tr(r, "Too many attempts. Wait a moment and try again."), http.StatusTooManyRequests)
		return
	}

	token := r.PathValue("token")
	if !utils.ValidateOpaqueID(token) {
		http.Error(w, "invalid invite token", http.StatusBadRequest)
		return
	}

	// We do NOT consume the token on GET; only on POST. Peek by checking
	// that ConsumeToken would succeed via a separate query would require a
	// new helper. For simplicity we just render the form; an invalid token
	// will be rejected on submit.
	data := struct {
		Authed bool
		User   db.User
		Token  string
		Error  string
		Config config.Config
		Locale string
	}{
		Token:  token,
		Config: *h.cfg,
		Locale: auth.RequestLocale(r),
	}
	err = h.templates(w, "signup.go.tmpl", data)
	if err != nil {
		log.Printf("template error in signup.go.tmpl: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// SignupSubmit consumes the invite token, creates the user, and starts a
// session. The invitee chooses username and password here; email comes from
// the invite token itself.
func (h *Handlers) SignupSubmit(w http.ResponseWriter, r *http.Request) {
	_, _, _, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		false, // check auth
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if h.rateLimited(w, r) {
		http.Error(w, tr(r, "Too many attempts. Wait a moment and try again."), http.StatusTooManyRequests)
		return
	}

	token := r.PathValue("token")
	if !utils.ValidateOpaqueID(token) {
		http.Error(w, "invalid invite token", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	confirm := r.FormValue("password_confirm")

	renderError := func(message string) {
		data := struct {
			Authed bool
			User   db.User
			Token  string
			Error  string
			Config config.Config
			Locale string
		}{
			Token:  token,
			Error:  message,
			Config: *h.cfg,
			Locale: auth.RequestLocale(r),
		}
		terr := h.templates(w, "signup.go.tmpl", data)
		if terr != nil {
			log.Printf("template error in signup.go.tmpl: %v", terr)
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	}

	if username == "" || password == "" {
		renderError(tr(r, "Enter a username and password."))
		return
	}

	if password != confirm {
		renderError(tr(r, "Passwords do not match."))
		return
	}

	err = db.IsValidUsername(username)
	if err != nil {
		renderError(tr(r, err.Error()))
		return
	}

	email, err := ConsumeInvite(token)
	if err != nil {
		log.Printf("signup: ConsumeInvite: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if email == "" {
		renderError(tr(r, "Invite link is invalid or expired."))
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		log.Printf("signup: HashPassword: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	u, err := db.Storage.CreateUser(username, email, hash, false)
	if err != nil {
		log.Printf("signup: CreateUser: %v", err)
		renderError(tr(r, "Could not create the user. The username may already be taken."))
		return
	}

	sid := utils.NewOpaqueID()
	session.Put(sid, *u)
	session.SetCookie(w, sid, h.cfg.SessionDuration)

	http.Redirect(w, r, h.cfg.BaseURL+"/", http.StatusFound)
}

// InvitesPage is the sysop-only admin panel that lists pending invites and
// hosts the form to mint new ones. The most recently created invite URL is
// surfaced via the "created" query param so the admin can copy it.
func (h *Handlers) InvitesPage(w http.ResponseWriter, r *http.Request) {
	u, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, // check auth
		true, // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !authed {
		return
	}
	if !u.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	data := struct {
		Authed      bool
		User        db.User
		Config      config.Config
		CurrentPage string
		Locale      string
		CreatedURL  string
		Error       string
	}{
		Authed:      true,
		User:        *u,
		Config:      *h.cfg,
		CurrentPage: "invites",
		Locale:      auth.RequestLocale(r),
		CreatedURL:  r.URL.Query().Get("created"),
		Error:       r.URL.Query().Get("error"),
	}

	err = h.templates(w, "tools_invites.go.tmpl", data)
	if err != nil {
		log.Printf("template error in tools_invites.go.tmpl: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// InviteCreate mints a new invite token for a given email and redirects back
// to /tools/invites with the URL embedded in the query string for copy/paste.
func (h *Handlers) InviteCreate(w http.ResponseWriter, r *http.Request) {
	u, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, // check auth
		true, // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !authed {
		return
	}
	if !u.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email == "" {
		http.Redirect(w, r,
			h.cfg.BaseURL+"/tools/invites?error="+url.QueryEscape(tr(r, "Enter an email.")),
			http.StatusFound)
		return
	}

	token, err := CreateInvite(email)
	if err != nil {
		log.Printf("invite: CreateInvite(%q): %v", email, err)
		msg := tr(r, "Could not create the invite.")
		if strings.Contains(err.Error(), "email") {
			// Validation problem with the address itself: show it verbatim.
			msg = err.Error()
		}
		http.Redirect(w, r,
			h.cfg.BaseURL+"/tools/invites?error="+url.QueryEscape(msg),
			http.StatusFound)
		return
	}

	inviteURL := h.cfg.BaseURL + "/signup/" + token
	http.Redirect(w, r,
		h.cfg.BaseURL+"/tools/invites?created="+url.QueryEscape(inviteURL),
		http.StatusFound)
}
