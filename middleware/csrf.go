package middleware

import (
	"net/http"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/log"
)

// CSRFProtection rejects unsafe (state-changing) cross-origin browser
// requests using the standard library's Sec-Fetch-Site / Origin checks.
// This is the engine-wide CSRF layer: it covers every POST route at once,
// complementing the SameSite=Lax session cookie and the double-submit
// tokens some screens also carry.
//
// Non-browser clients (curl, the REST API with bearer tokens) send neither
// Sec-Fetch-Site nor Origin and pass through untouched. The application's
// BaseURL is registered as a trusted origin so deployments behind a proxy
// keep working when the public origin differs from the Host header.
func CSRFProtection(next http.Handler) http.Handler {
	protection := http.NewCrossOriginProtection()
	if config.Cfg != nil && config.Cfg.BaseURL != "" {
		err := protection.AddTrustedOrigin(config.Cfg.BaseURL)
		if err != nil {
			log.Printf("csrf: invalid BaseURL %q as trusted origin: %v", config.Cfg.BaseURL, err)
		}
	}
	return protection.Handler(next)
}
