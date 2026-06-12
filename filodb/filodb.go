// Package filodb provides Filo builtins for database access.
// These builtins enable safe, parameterized SQL queries from Filo scripts.
//
// All SQL injection is prevented by requiring parameterized queries -
// the first argument is the SQL template with ? placeholders, and
// subsequent arguments are bound as parameters.
//
// Transaction support:
// - db-begin: Start transaction (errors if one already active)
// - db-commit: Commit and clear transaction (allows new begin)
// - db-rollback: Rollback and clear transaction (allows new begin)
//
// Query functions:
// - db-query: Returns list of rows (each row is a list)
// - db-query-one: Returns first row as list, or empty list
// - db-query-val: Returns scalar value from first row, first column
// - db-exec: Executes INSERT/UPDATE/DELETE
package filodb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/crgimenes/filo"
)

// DBStorage defines the database operations required by filodb.
// This interface is implemented by db.Store.
type DBStorage interface {
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) error
	BeginTransaction() (DBTransaction, error)
}

// DBTransaction defines transaction operations.
// This interface is implemented by db.Tx.
type DBTransaction interface {
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) error
	Commit() error
	Rollback() error
}

// FiloDBContext holds database state for a Filo script execution.
// It manages an optional transaction that persists across builtin calls.
type FiloDBContext struct {
	storage DBStorage
	tx      DBTransaction // nil when no transaction active
}

// NewContext creates a new FiloDBContext.
// If tx is provided, builtins will use it; otherwise they use storage directly.
func NewContext(storage DBStorage, tx DBTransaction) *FiloDBContext {
	return &FiloDBContext{
		storage: storage,
		tx:      tx,
	}
}

// NewContextWithStorage creates a context without an active transaction.
func NewContextWithStorage(storage DBStorage) *FiloDBContext {
	return &FiloDBContext{
		storage: storage,
		tx:      nil,
	}
}

// RegisterDBBuiltins adds database access functions to the engine.
func RegisterDBBuiltins(eng *filo.Engine, ctx *FiloDBContext) {
	eng.MustRegisterBuiltin("db-query", ctx.builtinDBQuery)
	eng.MustRegisterBuiltin("db-query-one", ctx.builtinDBQueryOne)
	eng.MustRegisterBuiltin("db-query-val", ctx.builtinDBQueryVal)
	eng.MustRegisterBuiltin("db-exec", ctx.builtinDBExec)
	eng.MustRegisterBuiltin("db-begin", ctx.builtinDBBegin)
	eng.MustRegisterBuiltin("db-commit", ctx.builtinDBCommit)
	eng.MustRegisterBuiltin("db-rollback", ctx.builtinDBRollback)
}

// builtinDBQuery executes a SELECT and returns all rows as a list of lists.
// Usage: (db-query "SELECT id, name FROM users WHERE age > ?" 18)
// Returns: ((1 "Alice") (2 "Bob"))
func (c *FiloDBContext) builtinDBQuery(ctx context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) < 1 {
		return filo.Value{}, fmt.Errorf("db-query expects at least 1 argument (sql)")
	}

	query, err := args[0].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-query: first argument must be string: %w", err)
	}

	params := argsToParams(args[1:])

	var rows *sql.Rows
	if c.tx != nil {
		rows, err = c.tx.Query(query, params...)
	} else {
		rows, err = c.storage.Query(query, params...)
	}
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-query: %w", err)
	}
	defer rows.Close()

	return rowsToList(rows)
}

// builtinDBQueryOne executes a SELECT and returns the first row as a list.
// Returns empty list if no rows found.
// Usage: (db-query-one "SELECT name, email FROM users WHERE id = ?" 42)
// Returns: ("Alice" "alice@example.com") or ()
func (c *FiloDBContext) builtinDBQueryOne(ctx context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) < 1 {
		return filo.Value{}, fmt.Errorf("db-query-one expects at least 1 argument (sql)")
	}

	query, err := args[0].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-query-one: first argument must be string: %w", err)
	}

	params := argsToParams(args[1:])

	var rows *sql.Rows
	if c.tx != nil {
		rows, err = c.tx.Query(query, params...)
	} else {
		rows, err = c.storage.Query(query, params...)
	}
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-query-one: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		// No rows - return empty list
		return filo.VList(nil), nil
	}

	return scanRowToList(rows)
}

// builtinDBQueryVal executes a SELECT and returns the first column of the first row.
// Returns empty list if no rows found.
// Usage: (db-query-val "SELECT COUNT(*) FROM users")
// Returns: 42
func (c *FiloDBContext) builtinDBQueryVal(ctx context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) < 1 {
		return filo.Value{}, fmt.Errorf("db-query-val expects at least 1 argument (sql)")
	}

	query, err := args[0].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-query-val: first argument must be string: %w", err)
	}

	params := argsToParams(args[1:])

	var rows *sql.Rows
	if c.tx != nil {
		rows, err = c.tx.Query(query, params...)
	} else {
		rows, err = c.storage.Query(query, params...)
	}
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-query-val: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		// No rows - return empty list
		return filo.VList(nil), nil
	}

	// Get column count
	cols, err := rows.Columns()
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-query-val: %w", err)
	}
	if len(cols) == 0 {
		return filo.VList(nil), nil
	}

	// Scan first column only
	var val any
	dest := make([]any, len(cols))
	dest[0] = &val
	for i := 1; i < len(cols); i++ {
		var discard any
		dest[i] = &discard
	}

	if err := rows.Scan(dest...); err != nil {
		return filo.Value{}, fmt.Errorf("db-query-val: %w", err)
	}

	return goToFiloValue(val), nil
}

// builtinDBExec executes INSERT/UPDATE/DELETE.
// Usage: (db-exec "INSERT INTO logs (msg) VALUES (?)" "hello")
// Returns: ()
func (c *FiloDBContext) builtinDBExec(ctx context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) < 1 {
		return filo.Value{}, fmt.Errorf("db-exec expects at least 1 argument (sql)")
	}

	query, err := args[0].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-exec: first argument must be string: %w", err)
	}

	params := argsToParams(args[1:])

	if c.tx != nil {
		err = c.tx.Exec(query, params...)
	} else {
		err = c.storage.Exec(query, params...)
	}
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-exec: %w", err)
	}

	return filo.VList(nil), nil
}

// builtinDBBegin starts a transaction.
// Errors if a transaction is already active.
// Usage: (db-begin)
func (c *FiloDBContext) builtinDBBegin(ctx context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) != 0 {
		return filo.Value{}, fmt.Errorf("db-begin expects 0 arguments")
	}

	if c.tx != nil {
		// Already in a transaction - error and rollback
		_ = c.tx.Rollback()
		c.tx = nil
		return filo.Value{}, fmt.Errorf("db-begin: transaction already active (rolled back)")
	}

	tx, err := c.storage.BeginTransaction()
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-begin: %w", err)
	}
	c.tx = tx

	return filo.VList(nil), nil
}

// builtinDBCommit commits the active transaction.
// Clears the transaction, allowing a new db-begin.
// Usage: (db-commit)
func (c *FiloDBContext) builtinDBCommit(ctx context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) != 0 {
		return filo.Value{}, fmt.Errorf("db-commit expects 0 arguments")
	}

	if c.tx == nil {
		return filo.Value{}, fmt.Errorf("db-commit: no active transaction")
	}

	err := c.tx.Commit()
	c.tx = nil // Clear tx even on error (it's no longer valid)
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-commit: %w", err)
	}

	return filo.VList(nil), nil
}

// builtinDBRollback rolls back the active transaction.
// Clears the transaction, allowing a new db-begin.
// Usage: (db-rollback)
func (c *FiloDBContext) builtinDBRollback(ctx context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) != 0 {
		return filo.Value{}, fmt.Errorf("db-rollback expects 0 arguments")
	}

	if c.tx == nil {
		return filo.Value{}, fmt.Errorf("db-rollback: no active transaction")
	}

	err := c.tx.Rollback()
	c.tx = nil // Clear tx
	if err != nil {
		return filo.Value{}, fmt.Errorf("db-rollback: %w", err)
	}

	return filo.VList(nil), nil
}

// Helper: convert Filo args to Go parameters
func argsToParams(args []filo.Value) []any {
	params := make([]any, len(args))
	for i, arg := range args {
		params[i] = filoToGoValue(arg)
	}
	return params
}

// Helper: convert Filo value to Go value for SQL parameter
func filoToGoValue(v filo.Value) any {
	switch v.Kind {
	case filo.KBool:
		return v.Bool
	case filo.KNumber:
		// Check if it's an integer
		if v.Num == float64(int64(v.Num)) {
			return int64(v.Num)
		}
		return v.Num
	case filo.KString:
		return v.Str
	default:
		return v.String()
	}
}

// Helper: convert Go value to Filo value
func goToFiloValue(v any) filo.Value {
	if v == nil {
		return filo.VString("")
	}
	switch val := v.(type) {
	case bool:
		return filo.VBool(val)
	case int64:
		return filo.VNum(float64(val))
	case float64:
		return filo.VNum(val)
	case string:
		return filo.VString(val)
	case []byte:
		return filo.VString(string(val))
	default:
		return filo.VString(fmt.Sprintf("%v", val))
	}
}

// Helper: convert sql.Rows to list of lists
func rowsToList(rows *sql.Rows) (filo.Value, error) {
	cols, err := rows.Columns()
	if err != nil {
		return filo.Value{}, err
	}

	var result []filo.Value
	for rows.Next() {
		rowVal, err := scanRowToListInternal(rows, len(cols))
		if err != nil {
			return filo.Value{}, err
		}
		result = append(result, rowVal)
	}

	if err := rows.Err(); err != nil {
		return filo.Value{}, err
	}

	return filo.VList(result), nil
}

// Helper: scan current row to list
func scanRowToList(rows *sql.Rows) (filo.Value, error) {
	cols, err := rows.Columns()
	if err != nil {
		return filo.Value{}, err
	}
	return scanRowToListInternal(rows, len(cols))
}

// Helper: scan row with known column count
func scanRowToListInternal(rows *sql.Rows, colCount int) (filo.Value, error) {
	values := make([]any, colCount)
	valuePtrs := make([]any, colCount)
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return filo.Value{}, err
	}

	result := make([]filo.Value, colCount)
	for i, v := range values {
		result[i] = goToFiloValue(v)
	}

	return filo.VList(result), nil
}
