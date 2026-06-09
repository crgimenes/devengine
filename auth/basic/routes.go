package basic

import "net/http"

// Routes registers basic auth's HTTP routes. Call once from the application's
// main, after db.Storage and templates are configured.
func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", h.LoginPage)
	mux.HandleFunc("POST /login", h.LoginSubmit)
	mux.HandleFunc("GET /signup/{token}", h.SignupPage)
	mux.HandleFunc("POST /signup/{token}", h.SignupSubmit)
	mux.HandleFunc("GET /tools/invites", h.InvitesPage)
	mux.HandleFunc("POST /tools/invites", h.InviteCreate)
}
