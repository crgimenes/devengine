# 1. Create a new application

**Goal:** get your own devengine app running — the one you'll build on in every
later tutorial.
**Before you start:** Go 1.26+ installed, and the `devengine` and `filo`
repositories checked out on disk.
**Time:** ~5 minutes.

A devengine app is tiny: load config, open a database, run the engine's
migrations, mount its routes, serve. The bootstrap tool writes that glue for
you — a config file and a ~100-line `main.go` — so you create the module and
run two commands.

## 1. Make the folder a module

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
git init
```

## 2. Point Go at the engine

Until devengine publishes a tagged release, your app builds against the local
checkouts through a Go workspace. From inside `inventory`, adjust the paths to
wherever you cloned them:

```sh
go work init . ../devengine ../filo
```

That's the only dependency wiring you need — no `require` lines, no
`go mod tidy`. The workspace supplies the engine.

## 3. Scaffold the app

```sh
go run github.com/crgimenes/devengine/cmd/devengine init --db inventory.db
```

This is the `devengine init` command (once the `devengine` tool is installed on
your `PATH`, it's simply `devengine init`). It:

- writes a starter **`init.filo`** (config) and **`main.go`** (the app),
- creates the **`inventory.db`** database and runs the engine migrations,
- asks for the first administrator — a username, an optional email, and a
  password. Remember them.

It never overwrites files you already have, so it's safe to re-run later just
to add another admin.

## 4. Run it

```sh
go run .
```

You'll see `serving on :3000`. Open <http://localhost:3000> and sign in with the
administrator you just created.

## Check it worked

After signing in you land on the home page with a **Tools** menu. Open
**Tools** — the sidebar (Database Schema, Forms, Menu Editor, …) is the whole
builder. The app is empty and ready.

## What you got

- **`init.filo`** — configuration: port, title, database file, UI language.
  Open it and change `SiteTitle` to taste; restart to apply.
- **`main.go`** — the ~100 lines that wire up the engine. Read the comments
  once; you rarely touch it.
- **`inventory.db`** — your data.

(`go.mod`, `go.work` and the git repo you made yourself in steps 1–2.)

## Tip: match the screenshots

These tutorials use the English interface. Open the user menu (top right) →
**My profile**, set **Language** to English, and save.

## Next

→ [2. Create your first table](02-create-a-table.md)
