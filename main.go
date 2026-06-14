// A minimal devengine application: load init.filo, open the database, run
// the engine migrations, wire the engine's routes, and serve. This is the
// whole app — devengine brings the admin tools, forms runtime, auth, file
// manager and REST API; your code just starts it.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/crgimenes/devengine/api"
	"github.com/crgimenes/devengine/assets/static"
	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/auth/basic"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/db/sqlite"
	_ "github.com/crgimenes/devengine/eav/ui/defaults" // register the field plugins
	"github.com/crgimenes/devengine/filemanager"
	"github.com/crgimenes/devengine/handlers"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/log"
	"github.com/crgimenes/devengine/middleware"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/devengine/templates"
	"github.com/crgimenes/filo"
)

// loadConfig fills config.Cfg from init.filo, falling back to the defaults
// set here when a key is absent.
func loadConfig(path string) {
	config.Cfg.GitTag = "dev"
	config.Cfg.Addrs = ":3000"
	config.Cfg.BaseURL = "http://localhost:3000"
	config.Cfg.DBFile = "app.db"
	config.Cfg.SiteTitle = "My App"

	F := filo.New()
	defer F.Close()
	F.SetGlobal("Address", config.Cfg.Addrs)
	F.SetGlobal("BaseURL", config.Cfg.BaseURL)
	F.SetGlobal("DBFile", config.Cfg.DBFile)
	F.SetGlobal("SiteTitle", config.Cfg.SiteTitle)
	F.SetGlobal("Locale", "en-US")

	src, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %v", path, err)
	}
	err = F.DoString(string(src))
	if err != nil {
		log.Fatalf("parse %s: %v", path, err)
	}

	config.Cfg.Addrs = F.MustGetString("Address")
	config.Cfg.BaseURL = F.MustGetString("BaseURL")
	config.Cfg.DBFile = F.MustGetString("DBFile")
	config.Cfg.SiteTitle = F.MustGetString("SiteTitle")
	i18n.SetLocale(F.MustGetString("Locale"))
}

func main() {
	loadConfig("init.filo")

	var err error
	db.Storage, err = sqlite.New()
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Storage.Close()

	err = sqlite.RunMigration()
	if err != nil {
		log.Fatalf("migrations: %v", err)
	}

	err = static.Init()
	if err != nil {
		log.Fatalf("static assets: %v", err)
	}

	h := handlers.New(handlers.Dependencies{
		Config:    config.Cfg,
		Templates: templates.ExecuteTemplate,
		FileUtilities: handlers.FileUtilities{
			Validate:     filemanager.ValidateFile,
			DataPath:     filemanager.DataFilePath,
			SaveMetadata: filemanager.SaveFileMetadata,
			NewFilename:  filemanager.FileName,
		},
	})
	basicAuth := basic.New(config.Cfg, templates.ExecuteTemplate)

	mux := http.NewServeMux()
	static.Routes(mux)
	auth.Routes(mux)
	session.Routes(mux)
	h.Routes(mux)
	filemanager.Routes(mux)
	api.Routes(mux)
	basicAuth.Routes(mux)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})

	srv := &http.Server{
		Addr:              config.Cfg.Addrs,
		Handler:           middleware.CSRFProtection(middleware.SecurityHeaders(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("serving on %s", config.Cfg.Addrs)
	err = srv.ListenAndServe()
	if err != nil {
		log.Fatalf("serve: %v", err)
	}
}
