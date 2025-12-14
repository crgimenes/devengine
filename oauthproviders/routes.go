package oauthproviders

import (
	"net/http"

	"github.com/crgimenes/devengine/config"
)

// Routes registra todas as rotas OAuth no mux.
func Routes(mux *http.ServeMux) {
	if config.Cfg.DiscordOAuthEnabled {
		mux.HandleFunc("/login/discord", DiscordProvider{}.LoginHandler)
		mux.HandleFunc("/discord/oauth/callback", DiscordProvider{}.CallbackHandler)
	}
	if config.Cfg.GithubOAuthEnabled {
		mux.HandleFunc("/login/github", GitHubProvider{}.LoginHandler)
		mux.HandleFunc("/github/oauth/callback", GitHubProvider{}.CallbackHandler)
	}
}
