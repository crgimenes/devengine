//go:build !dev

// Package assets embeds the engine's static assets (CSS, JS, vendored
// libraries) and serves them with ETag support; a dev build tag switches
// to reading from disk.
package assets

import (
	"embed"
	"mime"
	"net/http"
)

//go:embed *.css *.js bootstrap/css/*.css bootstrap/js/*.js bootstrap/js/*.js.map bootstrap/css/*.map bootstrap-icons/*.css bootstrap-icons/fonts/*.woff bootstrap-icons/fonts/*.woff2
var assets embed.FS

var FS = http.FS(assets)

func init() {
	// Ensure correct MIME types for certain assets.
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	//_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	_ = mime.AddExtensionType(".css", "text/css")
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".map", "application/json")
	_ = mime.AddExtensionType(".woff", "font/woff")
	_ = mime.AddExtensionType(".woff2", "font/woff2")

}
