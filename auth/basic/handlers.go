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

// LoginPage renders the login form.
func (h *Handlers) LoginPage(w http.ResponseWriter, r *http.Request) {
	_, _, _, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
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

	data := struct {
		Authed  bool
		User    db.User
		Error   string
		Message string
		Config  config.Config
	}{
		Authed: false,
		Config: *h.cfg,
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
		false, // check ratelimit (TODO)
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if username == "" || password == "" {
		h.renderLoginError(w, "Informe usuário e senha.")
		return
	}

	u, err := db.Storage.GetUserByUsername(username)
	if err != nil {
		log.Printf("login: GetUserByUsername(%q): %v", username, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if u == nil || u.PasswordHash == "" || !u.Enabled {
		h.renderLoginError(w, "Credenciais inválidas.")
		return
	}

	if !VerifyPassword(u.PasswordHash, password) {
		h.renderLoginError(w, "Credenciais inválidas.")
		return
	}

	sid := utils.NewOpaqueID()
	session.Put(sid, *u)
	session.SetCookie(w, sid, h.cfg.SessionDuration)

	http.Redirect(w, r, h.cfg.BaseURL+"/", http.StatusFound)
}

func (h *Handlers) renderLoginError(w http.ResponseWriter, message string) {
	data := struct {
		Authed  bool
		User    db.User
		Error   string
		Message string
		Config  config.Config
	}{
		Error:  message,
		Config: *h.cfg,
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
		false, // check ratelimit
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
		Token  string
		Error  string
		Config config.Config
	}{
		Token:  token,
		Config: *h.cfg,
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
		false, // check ratelimit
		true,  // prevent cache
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
			Token  string
			Error  string
			Config config.Config
		}{
			Token:  token,
			Error:  message,
			Config: *h.cfg,
		}
		terr := h.templates(w, "signup.go.tmpl", data)
		if terr != nil {
			log.Printf("template error in signup.go.tmpl: %v", terr)
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	}

	if username == "" || password == "" {
		renderError("Informe nome de usuário e senha.")
		return
	}

	if password != confirm {
		renderError("As senhas não conferem.")
		return
	}

	err = db.IsValidUsername(username)
	if err != nil {
		renderError(err.Error())
		return
	}

	email, err := ConsumeInvite(token)
	if err != nil {
		log.Printf("signup: ConsumeInvite: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if email == "" {
		renderError("Link de convite inválido ou expirado.")
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
		renderError("Não foi possível criar o usuário: " + err.Error())
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
		true,  // check auth
		false, // check ratelimit
		true,  // prevent cache
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
		Authed     bool
		User       db.User
		Config     config.Config
		CreatedURL string
		Error      string
	}{
		Authed:     true,
		User:       *u,
		Config:     *h.cfg,
		CreatedURL: r.URL.Query().Get("created"),
		Error:      r.URL.Query().Get("error"),
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
		true,  // check auth
		false, // check ratelimit
		true,  // prevent cache
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
			h.cfg.BaseURL+"/tools/invites?error="+url.QueryEscape("Informe um email."),
			http.StatusFound)
		return
	}

	token, err := CreateInvite(email)
	if err != nil {
		log.Printf("invite: CreateInvite(%q): %v", email, err)
		http.Redirect(w, r,
			h.cfg.BaseURL+"/tools/invites?error="+url.QueryEscape(err.Error()),
			http.StatusFound)
		return
	}

	inviteURL := h.cfg.BaseURL + "/signup/" + token
	http.Redirect(w, r,
		h.cfg.BaseURL+"/tools/invites?created="+url.QueryEscape(inviteURL),
		http.StatusFound)
}
