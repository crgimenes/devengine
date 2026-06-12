// Package db defines the storage contract of the engine: the Store and Tx
// interfaces, the record types they exchange, and database-agnostic helpers
// (hierarchical sorts, username validation, Filo script execution).
//
// Implementations live in subpackages — db/sqlite today — each with SQL
// written for its database; there is no generic SQL layer. Applications wire
// one implementation into the Storage global at boot.
package db

import (
	"database/sql"
	"errors"
)

// Row wraps sql.Row to keep a uniform return type and allow future extension.
// No context cancellation is used at this layer.
type Row struct {
	row *sql.Row
	err error
}

var (
	// Storage keeps a global handle for convenience. It holds whichever
	// Store implementation the application wired at boot (db/sqlite today).
	Storage Store

	ErrNoRows = sql.ErrNoRows
)

// NewRow wraps a sql.Row for return through the Store contract.
// Intended for Store implementations, not application code.
func NewRow(row *sql.Row) *Row {
	return &Row{row: row}
}

// NewErrorRow returns a Row that yields err on Scan. Intended for Store
// implementations reporting failures before a query runs.
func NewErrorRow(err error) *Row {
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
