// Package postgres implements the db.Store contract on PostgreSQL through
// jackc/pgx's database/sql adapter. SQL here is written for PostgreSQL
// ($N placeholders, RETURNING, tsvector search, plpgsql triggers) — there
// is no generic SQL layer shared with other backends.
//
// Unlike SQLite there is no writer/reader pool split: one pool serves
// both, and RW()/RO() return the same handle.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"runtime"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/utils"
)

// Postgres holds the connection pool.
type Postgres struct {
	pool *sql.DB
}

// Transaction wraps a write transaction.
type Transaction struct {
	tx *sql.Tx
}

// Tunables.
const (
	defaultConnMaxLife = 5 * time.Minute
	defaultPoolMinimum = 4 // raised to GOMAXPROCS if larger
)

// NewWithDSN opens a pool for the given PostgreSQL DSN
// (e.g. "postgres://user:pass@localhost:5432/devengine?sslmode=disable").
func NewWithDSN(dsn string) (*Postgres, error) {
	if dsn == "" {
		return nil, errors.New("database DSN required")
	}

	pool, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	max := defaultPoolMinimum
	if n := runtime.GOMAXPROCS(0); n > max {
		max = n
	}
	pool.SetMaxOpenConns(max)
	pool.SetMaxIdleConns(max)
	pool.SetConnMaxLifetime(defaultConnMaxLife)

	err = pool.Ping()
	if err != nil {
		utils.Closer(pool)
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

// BeginTransaction starts a write transaction.
func (s *Postgres) BeginTransaction() (db.Tx, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("db not initialized")
	}
	tx, err := s.pool.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	return &Transaction{tx: tx}, nil
}

// Commit finalizes a transaction; on error, attempts a rollback.
func (t *Transaction) Commit() error {
	if t == nil || t.tx == nil {
		return errors.New("nil tx")
	}
	err := t.tx.Commit()
	if err != nil {
		_ = t.tx.Rollback()
		t.tx = nil
		return err
	}
	t.tx = nil
	return nil
}

// Rollback aborts the transaction.
func (t *Transaction) Rollback() error {
	if t == nil || t.tx == nil {
		return nil
	}
	err := t.tx.Rollback()
	t.tx = nil
	return err
}

// Exec executes a write statement inside the transaction.
func (t *Transaction) Exec(query string, args ...any) error {
	if t == nil || t.tx == nil {
		return errors.New("nil tx")
	}
	_, err := t.tx.Exec(query, args...)
	return err
}

// Query runs a SELECT inside the transaction (consistent view).
func (t *Transaction) Query(query string, args ...any) (*sql.Rows, error) {
	if t == nil || t.tx == nil {
		return nil, errors.New("nil tx")
	}
	return t.tx.Query(query, args...)
}

// QueryRow returns a single row inside the transaction.
func (t *Transaction) QueryRow(query string, args ...any) *db.Row {
	if t == nil || t.tx == nil {
		return db.NewErrorRow(errors.New("nil tx"))
	}
	return db.NewRow(t.tx.QueryRow(query, args...))
}

// Exec executes a write statement on the pool.
func (s *Postgres) Exec(query string, args ...any) error {
	if s == nil || s.pool == nil {
		return errors.New("db not initialized")
	}
	_, err := s.pool.Exec(query, args...)
	return err
}

// Query executes a SELECT on the pool.
func (s *Postgres) Query(query string, args ...any) (*sql.Rows, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("db not initialized")
	}
	return s.pool.Query(query, args...)
}

// QueryRow executes a single-row SELECT on the pool.
func (s *Postgres) QueryRow(query string, args ...any) *db.Row {
	if s == nil || s.pool == nil {
		return db.NewErrorRow(errors.New("db not initialized"))
	}
	return db.NewRow(s.pool.QueryRow(query, args...))
}

// QueryRowRW is the same as QueryRow: PostgreSQL has no pool split.
func (s *Postgres) QueryRowRW(query string, args ...any) *db.Row {
	return s.QueryRow(query, args...)
}

// QueryRW is the same as Query: PostgreSQL has no pool split.
func (s *Postgres) QueryRW(query string, args ...any) (*sql.Rows, error) {
	return s.Query(query, args...)
}

// CheckpointWAL is a no-op: PostgreSQL manages its own WAL checkpoints.
func (s *Postgres) CheckpointWAL() error {
	return nil
}

// Close closes the pool.
func (s *Postgres) Close() {
	if s == nil {
		return
	}
	utils.Closer(s.pool)
}

// RW returns the underlying pool for external integrations.
func (s *Postgres) RW() *sql.DB {
	if s == nil {
		return nil
	}
	return s.pool
}

// RO returns the underlying pool for external integrations.
func (s *Postgres) RO() *sql.DB {
	if s == nil {
		return nil
	}
	return s.pool
}

// Compile-time checks: *Postgres is a db.Store and *Transaction is a db.Tx.
var (
	_ db.Store = (*Postgres)(nil)
	_ db.Tx    = (*Transaction)(nil)
)
