# 1. Create a new application

**Goal:** get your own devengine app running — the one you'll build on in every
later tutorial.
**Before you start:** Go 1.26+ installed, and the `devengine` and `filo`
repositories checked out on disk (they're developed together; until devengine
cuts a tagged release your app uses them straight from the workspace).
**Time:** ~10 minutes.

A devengine app is a thin program: load config, open a database, run the
engine's migrations, mount the engine's routes, serve. devengine brings
everything else — the admin tools, the forms runtime, authentication, the file
manager, the REST API. You write these ~100 lines once and never touch them
again.

## 1. Lay out the folders

Put your app next to the `devengine` and `filo` checkouts:

```
code/
├── devengine/
├── filo/
└── inventory/      ← your new app
```

```sh
mkdir inventory && cd inventory
go mod init github.com/you/inventory
```

## 2. Add the two files of glue

**`go.mod`** — add the dependencies (the workspace in step 4 supplies the
actual code, so the versions here are just names):

```
module github.com/you/inventory

go 1.26

require (
	github.com/crgimenes/devengine v0.0.6
	github.com/crgimenes/filo v0.0.9
)
```

**`init.filo`** — your app's configuration. Each `(set Key value)` overrides a
default:

```scheme
(set Address ":3000")
(set BaseURL "http://localhost:3000")
(set SiteTitle "Inventory")
(set DBFile "inventory.db")

;; UI language. The engine ships English (en-US) and a pt-BR dictionary.
(set Locale "en-US")
```

## 3. Write `main.go`

Paste this as-is. Read the comments once; you won't edit it again.

```go
// A minimal devengine application: load init.filo, open the database, run
// the engine migrations, wire the engine's routes, and serve.
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
```

## 4. Tie it to the local engine

From the **parent** folder, create a workspace so Go builds against your local
`devengine` and `filo` instead of trying to download them:

```sh
cd ..
go work init ./devengine ./filo ./inventory
cd inventory
```

(Already have a `go.work`? Just `go work use ./inventory`.)

## 5. Create the first sysop

The first administrator is created from the command line:

```sh
go run github.com/crgimenes/devengine/cmd/admin-bootstrap --db inventory.db
```

It asks for a username, an optional email, and a password. Remember them.

## 6. Run it

```sh
go run .
```

You'll see `serving on :3000`. Open <http://localhost:3000> and sign in with the
sysop you just created.

## Check it worked

After signing in you land on the home page with a **Tools** menu. Open
**Tools** — the sidebar (Database Schema, Forms, Menu Editor, …) is the whole
builder. The app is empty and ready.

## Tip: match the screenshots

These tutorials use the English interface. Open the user menu (top right) →
**My profile**, set **Language** to English, and save.

## Next

→ [2. Create your first table](02-create-a-table.md)
