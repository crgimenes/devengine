# PostgreSQL Storage Package

The `postgres` package implements the `db.Store` contract on PostgreSQL
through `jackc/pgx`'s `database/sql` adapter. All SQL here is written for
PostgreSQL — `$N` placeholders, `RETURNING`, `BOOLEAN` flags, `TIMESTAMPTZ`
timestamps, plpgsql triggers and a generated `tsvector` column for the file
search. There is no generic SQL layer shared with other backends.

## Usage

```go
import "github.com/crgimenes/devengine/db/postgres"

store, err := postgres.NewWithDSN("postgres://user:pass@localhost:5432/devengine?sslmode=disable")
if err != nil {
    // handle
}
db.Storage = store
err = postgres.RunMigration()
```

Unlike the SQLite backend there is no writer/reader pool split: a single
pool serves both, and `RW()`/`RO()` return the same handle.
`CheckpointWAL()` is a no-op — PostgreSQL manages its own WAL.

## Migrations

Engine migrations are embedded in this package (`*.up.sql`, PostgreSQL
dialect, ids `0001`–`0999` mirroring the SQLite set). Application
migrations register through `postgres.SetAppMigrationsFS` and use ids
`1000`–`9999`. The schema-drift warning stores its fingerprint in the
`devengine_meta` table (PostgreSQL has no `PRAGMA user_version`).

## Tests

`TestConformance` runs the shared `db/storetest` battery against a real
PostgreSQL started by testcontainers-go (test-only dependency; it never
enters application binaries). Migrations run once into a template database
and each subtest clones it with `CREATE DATABASE ... TEMPLATE` for cheap
isolation. Requirements: a running Docker daemon and the
`postgres:16-alpine` image (pulled automatically when absent). Skipped in
`-short` mode and when Docker is unavailable.
