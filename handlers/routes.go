package handlers

import "net/http"

// Routes registra as rotas de páginas no mux.
func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.Home)
	mux.HandleFunc("/login", h.LoginPage)
	mux.HandleFunc("POST /login/magic_link", h.LoginMagic)
	mux.HandleFunc("GET /link/{token}", h.MagicLink)
	mux.HandleFunc("/me", h.Profile)
	mux.HandleFunc("/tools", h.Tools)
}
