package postgres

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/db/storetest"
)

// TestConformance runs the shared db.Store battery against a real PostgreSQL
// in a container. Migrations run once into a template database; each subtest
// clones it with CREATE DATABASE ... TEMPLATE for cheap isolation.
func TestConformance(t *testing.T) {
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

	baseDSN, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	// Admin pool on the default database creates the per-test clones.
	admin, err := NewWithDSN(baseDSN)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(admin.Close)

	// Migrate the template database once.
	err = admin.Exec("CREATE DATABASE tpl")
	if err != nil {
		t.Fatalf("create template db: %v", err)
	}
	tpl, err := NewWithDSN(withDatabase(t, baseDSN, "tpl"))
	if err != nil {
		t.Fatalf("open template db: %v", err)
	}
	err = RunMigrationOn(tpl)
	if err != nil {
		tpl.Close()
		t.Fatalf("migrate template db: %v", err)
	}
	tpl.Close() // template must have no active connections to be cloned

	n := 0
	storetest.Run(t, func(t *testing.T) db.Store {
		n++
		name := fmt.Sprintf("conf_%d", n)
		err := admin.Exec("CREATE DATABASE " + name + " TEMPLATE tpl")
		if err != nil {
			t.Fatalf("clone template db: %v", err)
		}
		s, err := NewWithDSN(withDatabase(t, baseDSN, name))
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		t.Cleanup(s.Close)
		return s
	})
}

// withDatabase swaps the database name in a postgres:// DSN.
func withDatabase(t *testing.T, dsn, dbname string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	u.Path = "/" + dbname
	return u.String()
}
