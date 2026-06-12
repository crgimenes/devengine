package postgres

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Application migrations (ids 1000+) in PostgreSQL dialect apply on top of
// the unified engine schema, against a real container.
func TestAppMigrationsApply(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping container test in -short mode")
	}
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("devengine"),
		tcpostgres.WithUsername("devengine"),
		tcpostgres.WithPassword("devengine"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Skipf("could not start postgres container (docker running?): %v", err)
	}
	t.Cleanup(func() {
		err := testcontainers.TerminateContainer(ctr)
		if err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	s, err := NewWithDSN(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(s.Close)

	appFS := fstest.MapFS{
		"1000_posts.up.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE app_posts (
				id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
				author_id BIGINT NOT NULL REFERENCES users(id),
				title TEXT NOT NULL
			);`)},
		"1001_posts_index.up.sql": &fstest.MapFile{Data: []byte(`
			CREATE INDEX idx_app_posts_author ON app_posts(author_id);`)},
	}
	SetAppMigrationsFS(appFS)
	t.Cleanup(func() { SetAppMigrationsFS(nil) })

	err = RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}

	u, err := s.CreateUser("autor", "autor@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	err = s.Exec("INSERT INTO app_posts (author_id, title) VALUES ($1, $2)", u.ID, "hello")
	if err != nil {
		t.Fatalf("insert into app table: %v", err)
	}

	err = RunMigrationOn(s)
	if err != nil {
		t.Fatalf("second RunMigrationOn: %v", err)
	}

	tables, err := s.ListRelationalTables()
	if err != nil {
		t.Fatalf("ListRelationalTables: %v", err)
	}
	seen := false
	for _, tb := range tables {
		if tb.Name == "app_posts" {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("app_posts missing from schema listing: %v", tables)
	}
}
