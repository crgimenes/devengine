// Package db provides SQLite access using modernc.org/sqlite (no CGO).
// Goals: performance, concurrency and predictability with minimal dependencies.
// - Separate pools: one writer (RW) and many readers (RO).
// - WAL + synchronous=NORMAL + busy_timeout.
// - Short transactions; no per-operation context timeouts in this layer.
// - WAL checkpoint on Close() for hygiene.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"time"

	_ "modernc.org/sqlite"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/log"
	"github.com/crgimenes/devengine/utils"
)

// SQLite holds separate read/write pools.
type SQLite struct {
	rw *sql.DB // single-writer pool
	ro *sql.DB // read-only pool
}

// Transaction wraps a write transaction.
type Transaction struct {
	tx *sql.Tx
}

// Row wraps sql.Row to keep a uniform return type and allow future extension.
// No context cancellation is used at this layer.
type Row struct {
	row *sql.Row
	err error
}

var (
	// Storage keeps a global handle for convenience (preserves your original pattern).
	Storage *SQLite

	ErrNoRows = sql.ErrNoRows
)

func newRow(row *sql.Row) *Row {
	return &Row{row: row}
}

func errorRow(err error) *Row {
	return &Row{err: err}
}

// Scan delegates to the underlying sql.Row.
func (r *Row) Scan(dest ...any) error {
	if r == nil {
		return errors.New("nil row")
	}
	if r.err != nil {
		return r.err
	}
	if r.row == nil {
		return errors.New("nil row")
	}
	return r.row.Scan(dest...)
}

// Err mirrors (*sql.Row).Err.
func (r *Row) Err() error {
	if r == nil {
		return errors.New("nil row")
	}
	if r.err != nil {
		return r.err
	}
	if r.row == nil {
		return errors.New("nil row")
	}
	return r.row.Err()
}

// Tunables (adjust as needed for your service profile).
const (
	// Slightly longer to avoid flakiness with parallel tests/CI.
	defaultBusyTimeout     = 15 * time.Second
	defaultConnMaxLifeRW   = 2 * time.Minute
	defaultConnMaxLifeRO   = 5 * time.Minute
	defaultReadPoolMinimum = 4 // will be raised to GOMAXPROCS if larger
)

// New initializes RW/RO pools.
// Uses config.Cfg.DBFile as the SQLite path/URI;
func New() (*SQLite, error) {
	path := config.Cfg.DBFile
	return NewWithPath(path)
}

// NewWithPath creates SQLite pools for a specific file/URI path.
func NewWithPath(path string) (*SQLite, error) {
	if path == "" {
		return nil, errors.New("database path required")
	}

	// DSN for write pool: WAL, NORMAL, busy_timeout, foreign_keys ON, automatic_index ON,
	// temp_store in memory, modest cache, and tx lock set to IMMEDIATE.
	rwDSN := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(%d)&_pragma=foreign_keys(ON)&_pragma=automatic_index(ON)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-20000)&_txlock=immediate",
		path, int(defaultBusyTimeout.Milliseconds()),
	)
	// DSN for read-only pool: mode=ro with busy_timeout and foreign_keys ON.
	roDSN := fmt.Sprintf(
		"file:%s?mode=ro&_pragma=busy_timeout(%d)&_pragma=foreign_keys(ON)",
		path, int(defaultBusyTimeout.Milliseconds()),
	)

	s := &SQLite{}

	// Open writer (single connection for predictable write latency under contention).
	rw, err := sql.Open("sqlite", rwDSN)
	if err != nil {
		return nil, fmt.Errorf("open RW: %w", err)
	}
	rw.SetMaxOpenConns(1)
	rw.SetMaxIdleConns(1)
	rw.SetConnMaxLifetime(defaultConnMaxLifeRW)
	s.rw = rw

	// Open readers (parallel reads).
	ro, err := sql.Open("sqlite", roDSN)
	if err != nil {
		utils.Closer(s.rw)
		return nil, fmt.Errorf("open RO: %w", err)
	}
	max := defaultReadPoolMinimum
	if n := runtime.GOMAXPROCS(0); n > max {
		max = n
	}
	ro.SetMaxOpenConns(max)
	ro.SetMaxIdleConns(max)
	ro.SetConnMaxLifetime(defaultConnMaxLifeRO)
	s.ro = ro

	return s, nil
}

// BeginTransaction starts a write transaction.
//
// IMPORTANT: We intentionally do not propagate context timeouts in this package.
// Callers should avoid long-lived transactions; SQLite busy_timeout handles
// transient contention, and application code should keep critical sections short.
func (s *SQLite) BeginTransaction() (*Transaction, error) {
	if s == nil || s.rw == nil {
		return nil, errors.New("db not initialized")
	}
	tx, err := s.rw.BeginTx(context.Background(), nil)
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
	// Note: Do not use a request-scoped context here; the lifetime of sql.Rows
	// extends beyond this function, and premature cancellation would break iteration.
	return t.tx.Query(query, args...)
}

// QueryRow returns a single row inside the transaction.
func (t *Transaction) QueryRow(query string, args ...any) *Row {
	if t == nil || t.tx == nil {
		return errorRow(errors.New("nil tx"))
	}
	return newRow(t.tx.QueryRow(query, args...))
}

// Exec executes a write statement on the RW pool (outside explicit transactions).
func (s *SQLite) Exec(query string, args ...any) error {
	if s == nil || s.rw == nil {
		return errors.New("db not initialized")
	}
	_, err := s.rw.Exec(query, args...)
	return err
}

// Query executes a SELECT on the RO pool (parallel reads).
func (s *SQLite) Query(query string, args ...any) (*sql.Rows, error) {
	if s == nil || s.ro == nil {
		return nil, errors.New("db not initialized")
	}
	// Note: Avoid wrapping with request-scoped contexts here. The returned
	// sql.Rows must remain valid for iteration by the caller.
	return s.ro.Query(query, args...)
}

// QueryRow executes a single-row SELECT on the RO pool.
func (s *SQLite) QueryRow(query string, args ...any) *Row {
	if s == nil || s.ro == nil {
		return errorRow(errors.New("db not initialized"))
	}
	return newRow(s.ro.QueryRow(query, args...))
}

func (s *SQLite) QueryRowRW(query string, args ...any) *Row {
	if s == nil || s.rw == nil {
		return errorRow(errors.New("db not initialized"))
	}
	return newRow(s.rw.QueryRow(query, args...))
}

// QueryRW allows SELECT using the RW pool (rarely needed).
func (s *SQLite) QueryRW(query string, args ...any) (*sql.Rows, error) {
	if s == nil || s.rw == nil {
		return nil, errors.New("db not initialized")
	}
	// See note above: keep rows consumable without premature cancellation.
	return s.rw.Query(query, args...)
}

// CheckpointWAL triggers a WAL checkpoint with TRUNCATE.
func (s *SQLite) CheckpointWAL() error {
	if s == nil || s.rw == nil {
		return errors.New("db not initialized")
	}
	_, err := s.rw.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

// Close closes pools; performs a best-effort WAL checkpoint first.
func (s *SQLite) Close() {
	if s == nil {
		return
	}
	if err := s.CheckpointWAL(); err != nil {
		log.Println("wal checkpoint:", err)
	}
	utils.Closer(s.ro)
	utils.Closer(s.rw)
}
