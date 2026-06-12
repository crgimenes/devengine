# Contributing to devengine

devengine is a reusable Go engine for data-driven web systems: you define
tables (EAV entity types), build forms and search forms over them, wire
menus, and attach behavior with the Filo language — validation rules,
computed fields, buttons. Applications (edev, rpgstudios, tarefas, pedidos)
consume the engine; the engine never imports them.

The philosophy in three lines:

- **Stdlib first.** New dependencies need a strong reason. Test-only
  dependencies are acceptable; binary dependencies rarely are.
- **Naturally light.** No speculative caching or cleverness. Any
  optimization must prove itself with a benchmark before it lands.
- **Teaching example.** This codebase doubles as study material for a Go
  study group. Clarity beats brevity.

## Repository layout

```
devengine/
├── assets/          embedded static files + the /assets handler (ETag, 304)
├── auth/            session prelude, request locale; auth/basic = login/invites
├── cmd/             CLIs: admin-bootstrap (first sysop), snapshot (backups)
├── config/          process configuration (populated from init.filo by apps)
├── db/              SQLite storage, migrations (*.up.sql), all SQL lives here
├── eav/ui/          field plugin registry + one package per plugin
├── filemanager/     user file uploads, quota
├── filo*/           Filo builtin packages (filodb, filoeav, filofile, ...)
├── handlers/        every HTTP handler (admin tools + runtime)
├── i18n/            translations: dictionaries, overrides, user content
├── log/             leveled colored logger, optional JSON (slog)
├── middleware/      security headers
├── ratelimit/       token bucket per client IP
├── session/         in-memory sessions, CSRF helpers
└── templates/       html/template files (*.go.tmpl) + render helpers
```

An application is a thin main: load `init.filo`, open the database, run
migrations, wire the engine routes, serve.

## Getting started

```sh
cd devengine && go test ./...        # everything runs on a throwaway SQLite

cd ../pedidos                        # or any sibling app
go build -trimpath -o pedidos .
go run github.com/crgimenes/devengine/cmd/admin-bootstrap -db pedidos.db
./pedidos                            # http://localhost:3233, config in init.filo
```

Always build with `-trimpath`.

## The verify flow

Run before considering any change done. All of them must come back clean —
zero issues, zero failures:

```sh
go fix ./...
go vet ./...
staticcheck ./...
golangci-lint run     # includes gocyclo, threshold 30, tests exempt
gosec ./...           # false positives get // #nosec RULE -- justification
go test ./...
```

## Conventions

- **Code is US English.** Identifiers, comments, internal errors — even in
  examples. The only user-visible text is i18n keys (see below).
- **Error handling style.** Assignment and check on separate lines:

  ```go
  err := doThing()
  if err != nil {
      return err
  }
  ```

  Not `if err := doThing(); err != nil`. Function length is not a metric
  here (Go is verbose); cyclomatic complexity is, enforced by golangci-lint.
- **SQL with numbered columns.** Any statement with more than one column
  aligns column ↔ placeholder ↔ struct field with `-- N` / `// N` comments.
  Look at any function in `db/sqlite/forms.go` for the shape.
- **Solve it in SQL, not in Go.** Sorting, filtering and pagination belong
  in the query (`ORDER BY`, `WHERE`, `LIMIT`), not in Go code after the
  scan. If the database can answer the question, let it. Exceptions:
  recursive tree shaping of small, already-loaded sets (menu items, form
  elements) and Filo `pos_load` decoration.
- **Migrations: one unified schema per backend until 1.0.0.** The engine
  ships a single `0001_schema.up.sql` in `db/sqlite` and `db/postgres`;
  schema changes edit it in place and development databases are recreated
  from scratch (the boot drift warning reminds you). Append-only history
  starts at 1.0.0. Application migrations use ids `1000`-`9999` and apply
  on top.
- **Logging** goes through `github.com/crgimenes/devengine/log` (never the
  stdlib logger directly). Structured fields: `log.Errorw("msg", "k", v)`.
  Never log passwords, tokens or secrets.
- **Errors shown to users** never carry `err.Error()`. Log the detail under
  a reference id (`logRef`) and show a generic message with the id.
- **Listings** use ordered cursor-based infinite scroll
  (`WHERE id < ? ORDER BY id DESC`), not OFFSET pagination.

## i18n

The US English string IS the key. Handlers translate with the request
locale; templates receive the locale through the page data:

```go
tr(r, "Record created successfully")          // in handlers
```

```html
{{t $.Locale "Sign in"}}                      <!-- in templates -->
```

Adding a user-facing string means adding the English key at the call site
and its translation to `i18n/pt_br.go`. Untranslated keys fall back to
English — never break a page over a missing entry. Runtime screens also
translate user content (form labels, menu items) via `i18n.ContentOr`;
authoring screens always show the original text.

## Adding a field plugin

A field plugin owns the parsing/validation of one `ui_kind`. Today it
takes three steps (roadmap item 8.2 will collapse them into one package):

1. Create `eav/ui/<kind>/<kind>.go` implementing `ui.FieldUI` and
   registering itself:

   ```go
   func init() {
       ui.Register("mykind", func() ui.FieldUI { return Plugin{} })
   }
   ```

   See `eav/ui/text/text.go` for the complete shape (ID, PrimitiveKinds,
   ParseOptions, Parse, Validate).
2. Add the render partial `templates/partials/field_<kind>.go.tmpl`
   (define `field_<kind>`). The runtime dispatches by name and falls back
   to `field_text` for unknown kinds.
3. Add the option defaults to `getFieldDefaults` in
   `templates/templates.go` so `ui_meta_json` merges over sane values.

Import the package for side effects where plugins are wired
(`eav/ui/defaults`). Write table-driven tests for Parse/Validate next to
the plugin.

## Adding a Filo builtin

Filo scripts (pre_save, pos_load, validate_expr, computed_expr, buttons,
menu actions, REPL) see whatever builtins each context registers.

1. Create or extend a `filo<domain>` package exposing
   `RegisterBuiltins(eng *filo.Engine, ...)` (see `filoeav` for one that
   carries a transaction).
2. Register it in each execution context that should see it — search
   `handlers/` for the existing `filostrings.RegisterBuiltins` calls; the
   button runner and the REPL each list their builtin set explicitly.
3. Builtins that write inside a save must join the surrounding transaction
   (see `filoeav.NewContextTx`) so a script error rolls everything back.

## Adding an admin screen

Handler in `handlers/` (use `auth.Prelude` + the sysop check), template in
`templates/`, route in `handlers/routes.go`, then the three navigation
spots: the card in `templates/tools.go.tmpl`, the sidebar in
`templates/partials/tools_sidebar.go.tmpl` and the dropdown in
`templates/partials/tools_menu.go.tmpl`.

## Testing

- The handler harness (`newHTTPTestEnv` in
  `handlers/render_httptest_test.go`) runs the REAL embedded templates
  over a throwaway database — it catches handler↔template contract bugs
  that stub templates cannot see. `TestEndToEndFlow` in
  `handlers/e2e_httptest_test.go` is the executable narrative of the whole
  engine: login → schema → form → record → validation → listing.
- Database tests open a real file under `t.TempDir()`. Never `:memory:`.
- If you optimize something, bring the before/after benchmark
  (`log/bench_test.go` and `assets/static/static_test.go` show the
  pattern).
