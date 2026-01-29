package filodb

import (
	"database/sql"

	"github.com/crgimenes/filo"
	"github.com/crgimenes/filo/filostrings"
)

// SQLiteAdapter adapts a *sql.DB pair (rw and ro) to the DBStorage interface.
// This is the standard way to connect filodb to a SQLite database.
type SQLiteAdapter struct {
	rw *sql.DB // read-write connection
	ro *sql.DB // read-only connection (for queries)
}

// NewSQLiteAdapter creates an adapter from rw and ro database connections.
// The ro connection is used for queries, rw for writes and transactions.
func NewSQLiteAdapter(rw, ro *sql.DB) *SQLiteAdapter {
	return &SQLiteAdapter{rw: rw, ro: ro}
}

func (a *SQLiteAdapter) Query(query string, args ...any) (*sql.Rows, error) {
	return a.ro.Query(query, args...)
}

func (a *SQLiteAdapter) Exec(query string, args ...any) error {
	_, err := a.rw.Exec(query, args...)
	return err
}

func (a *SQLiteAdapter) BeginTransaction() (DBTransaction, error) {
	tx, err := a.rw.Begin()
	if err != nil {
		return nil, err
	}
	return &sqlTxAdapter{tx: tx}, nil
}

// sqlTxAdapter adapts *sql.Tx to DBTransaction
type sqlTxAdapter struct {
	tx *sql.Tx
}

func (t *sqlTxAdapter) Query(query string, args ...any) (*sql.Rows, error) {
	return t.tx.Query(query, args...)
}

func (t *sqlTxAdapter) Exec(query string, args ...any) error {
	_, err := t.tx.Exec(query, args...)
	return err
}

func (t *sqlTxAdapter) Commit() error {
	return t.tx.Commit()
}

func (t *sqlTxAdapter) Rollback() error {
	return t.tx.Rollback()
}

// MakeScriptSetupFunc creates a script setup function that registers both
// string and DB builtins. Use this with db.CurrentScriptSetup.
//
// Example usage in your application's init:
//
//	func init() {
//	    adapter := filodb.NewSQLiteAdapter(db.Storage.RW(), db.Storage.RO())
//	    db.CurrentScriptSetup = filodb.MakeScriptSetupFunc(adapter)
//	}
func MakeScriptSetupFunc(storage DBStorage) func(*filo.Engine) {
	return func(eng *filo.Engine) {
		// Register string builtins
		filostrings.RegisterBuiltins(eng)

		// Register DB builtins
		dbCtx := NewContextWithStorage(storage)
		RegisterDBBuiltins(eng, dbCtx)
	}
}
