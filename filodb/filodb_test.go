package filodb

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/crgimenes/filo"
	_ "modernc.org/sqlite" // SQLite driver
)

// testDB creates a temporary SQLite database for testing
func testDB(t *testing.T) *sql.DB {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create test table
	_, err = db.Exec(`CREATE TABLE test_users (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		age INTEGER,
		balance REAL
	)`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`INSERT INTO test_users (id, name, age, balance) VALUES
		(1, 'Alice', 30, 100.50),
		(2, 'Bob', 25, 200.75),
		(3, 'Charlie', 35, 50.00)`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		_ = os.RemoveAll(dir)
	})

	return db
}

// testStorageAdapter implements DBStorage using a raw *sql.DB for testing
type testStorageAdapter struct {
	db *sql.DB
}

func (a *testStorageAdapter) Query(query string, args ...any) (*sql.Rows, error) {
	return a.db.Query(query, args...)
}

func (a *testStorageAdapter) Exec(query string, args ...any) error {
	_, err := a.db.Exec(query, args...)
	return err
}

func (a *testStorageAdapter) BeginTransaction() (DBTransaction, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	return &testTransactionAdapter{tx: tx}, nil
}

// testTransactionAdapter implements DBTransaction using a raw *sql.Tx
type testTransactionAdapter struct {
	tx *sql.Tx
}

func (t *testTransactionAdapter) Query(query string, args ...any) (*sql.Rows, error) {
	return t.tx.Query(query, args...)
}

func (t *testTransactionAdapter) Exec(query string, args ...any) error {
	_, err := t.tx.Exec(query, args...)
	return err
}

func (t *testTransactionAdapter) Commit() error {
	return t.tx.Commit()
}

func (t *testTransactionAdapter) Rollback() error {
	return t.tx.Rollback()
}

// setupTestStorage creates test storage
func setupTestStorage(t *testing.T) DBStorage {
	t.Helper()
	db := testDB(t)
	return &testStorageAdapter{db: db}
}

// createEngine creates a Filo engine with DB builtins registered
func createEngine(t *testing.T, storage DBStorage) (*filo.Engine, *FiloDBContext) {
	t.Helper()
	eng := filo.NewEngine()
	ctx := NewContextWithStorage(storage)
	RegisterDBBuiltins(eng, ctx)
	return eng, ctx
}

func runScript(t *testing.T, eng *filo.Engine, script string) filo.Value {
	t.Helper()
	ctx := context.Background()
	cfg := filo.EvalConfig{
		StepLimit:      10000,
		RecursionLimit: 100,
		Timeout:        5 * time.Second,
	}

	result, _, err := eng.RunScript(ctx, script, nil, cfg)
	if err != nil {
		t.Fatalf("Script failed: %v\nScript: %s", err, script)
	}
	return result
}

func runScriptExpectError(t *testing.T, eng *filo.Engine, script string) error {
	t.Helper()
	ctx := context.Background()
	cfg := filo.EvalConfig{
		StepLimit:      10000,
		RecursionLimit: 100,
		Timeout:        5 * time.Second,
	}

	_, _, err := eng.RunScript(ctx, script, nil, cfg)
	return err
}

// ========================================
// db-query tests
// ========================================

func TestDBQuery_MultipleRows(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	result := runScript(t, eng, `(db-query "SELECT id, name FROM test_users ORDER BY id")`)

	list, err := result.AsList()
	if err != nil {
		t.Fatalf("Expected list, got %v", result)
	}
	if len(list) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(list))
	}

	// Check first row
	row, _ := list[0].AsList()
	if len(row) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(row))
	}
}

func TestDBQuery_WithParams(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	result := runScript(t, eng, `(db-query "SELECT name FROM test_users WHERE age > ?" 28)`)

	list, _ := result.AsList()
	if len(list) != 2 { // Alice (30) and Charlie (35)
		t.Errorf("Expected 2 rows, got %d", len(list))
	}
}

func TestDBQuery_EmptyResult(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	result := runScript(t, eng, `(db-query "SELECT name FROM test_users WHERE age > 100")`)

	list, _ := result.AsList()
	if len(list) != 0 {
		t.Errorf("Expected 0 rows, got %d", len(list))
	}
}

// ========================================
// db-query-one tests
// ========================================

func TestDBQueryOne_Found(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	result := runScript(t, eng, `(db-query-one "SELECT name, age FROM test_users WHERE id = ?" 1)`)

	list, err := result.AsList()
	if err != nil {
		t.Fatalf("Expected list, got %v", result)
	}
	if len(list) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(list))
	}

	name, _ := list[0].AsString()
	if name != "Alice" {
		t.Errorf("Expected 'Alice', got '%s'", name)
	}
}

func TestDBQueryOne_NotFound(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	result := runScript(t, eng, `(db-query-one "SELECT name FROM test_users WHERE id = 999")`)

	list, _ := result.AsList()
	if len(list) != 0 {
		t.Errorf("Expected empty list, got %d elements", len(list))
	}
}

// ========================================
// db-query-val tests
// ========================================

func TestDBQueryVal_Count(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	result := runScript(t, eng, `(db-query-val "SELECT COUNT(*) FROM test_users")`)

	num, err := result.AsNumber()
	if err != nil {
		t.Fatalf("Expected number, got %v", result)
	}
	if num != 3 {
		t.Errorf("Expected 3, got %v", num)
	}
}

func TestDBQueryVal_String(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	result := runScript(t, eng, `(db-query-val "SELECT name FROM test_users WHERE id = 2")`)

	str, err := result.AsString()
	if err != nil {
		t.Fatalf("Expected string, got %v", result)
	}
	if str != "Bob" {
		t.Errorf("Expected 'Bob', got '%s'", str)
	}
}

func TestDBQueryVal_NotFound(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	result := runScript(t, eng, `(db-query-val "SELECT name FROM test_users WHERE id = 999")`)

	list, _ := result.AsList()
	if len(list) != 0 {
		t.Errorf("Expected empty list for not found, got: %v", result)
	}
}

func TestDBQueryVal_InExpression(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	// Use query result directly in condition
	result := runScript(t, eng, `
		(if (> (db-query-val "SELECT balance FROM test_users WHERE id = 1") 50)
			"high"
			"low")
	`)

	str, _ := result.AsString()
	if str != "high" {
		t.Errorf("Expected 'high', got '%s'", str)
	}
}

// ========================================
// db-exec tests
// ========================================

func TestDBExec_Insert(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	runScript(t, eng, `(db-exec "INSERT INTO test_users (name, age) VALUES (?, ?)" "Dave" 40)`)

	// Verify insertion
	result := runScript(t, eng, `(db-query-val "SELECT COUNT(*) FROM test_users")`)
	num, _ := result.AsNumber()
	if num != 4 {
		t.Errorf("Expected 4 users after insert, got %v", num)
	}
}

func TestDBExec_Update(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	runScript(t, eng, `(db-exec "UPDATE test_users SET age = 31 WHERE id = 1")`)

	result := runScript(t, eng, `(db-query-val "SELECT age FROM test_users WHERE id = 1")`)
	num, _ := result.AsNumber()
	if num != 31 {
		t.Errorf("Expected age 31, got %v", num)
	}
}

// ========================================
// Transaction tests
// ========================================

func TestTransaction_CommitPersists(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	runScript(t, eng, `
		(db-begin)
		(db-exec "INSERT INTO test_users (name, age) VALUES ('Eve', 22)")
		(db-commit)
	`)

	result := runScript(t, eng, `(db-query-val "SELECT COUNT(*) FROM test_users WHERE name = 'Eve'")`)
	num, _ := result.AsNumber()
	if num != 1 {
		t.Errorf("Expected 1 after commit, got %v", num)
	}
}

func TestTransaction_RollbackDiscards(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	runScript(t, eng, `
		(db-begin)
		(db-exec "INSERT INTO test_users (name, age) VALUES ('Frank', 28)")
		(db-rollback)
	`)

	result := runScript(t, eng, `(db-query-val "SELECT COUNT(*) FROM test_users WHERE name = 'Frank'")`)
	num, _ := result.AsNumber()
	if num != 0 {
		t.Errorf("Expected 0 after rollback, got %v", num)
	}
}

func TestTransaction_CanBeginAfterCommit(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	// First transaction
	runScript(t, eng, `
		(db-begin)
		(db-exec "INSERT INTO test_users (name, age) VALUES ('First', 1)")
		(db-commit)
	`)

	// Second transaction after commit
	runScript(t, eng, `
		(db-begin)
		(db-exec "INSERT INTO test_users (name, age) VALUES ('Second', 2)")
		(db-commit)
	`)

	result := runScript(t, eng, `(db-query-val "SELECT COUNT(*) FROM test_users WHERE name IN ('First', 'Second')")`)
	num, _ := result.AsNumber()
	if num != 2 {
		t.Errorf("Expected 2 records from two transactions, got %v", num)
	}
}

func TestTransaction_CanBeginAfterRollback(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	// First transaction - rollback
	runScript(t, eng, `
		(db-begin)
		(db-exec "INSERT INTO test_users (name, age) VALUES ('Temp', 0)")
		(db-rollback)
	`)

	// Second transaction - commit
	runScript(t, eng, `
		(db-begin)
		(db-exec "INSERT INTO test_users (name, age) VALUES ('Final', 99)")
		(db-commit)
	`)

	result := runScript(t, eng, `(db-query-val "SELECT COUNT(*) FROM test_users WHERE name = 'Final'")`)
	num, _ := result.AsNumber()
	if num != 1 {
		t.Errorf("Expected 1 after second transaction, got %v", num)
	}
}

// ========================================
// Error cases
// ========================================

func TestDBBegin_NestedError(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	err := runScriptExpectError(t, eng, `
		(db-begin)
		(db-begin)
	`)

	if err == nil {
		t.Error("Expected error for nested db-begin")
	}
}

func TestDBCommit_NoTransaction(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	err := runScriptExpectError(t, eng, `(db-commit)`)

	if err == nil {
		t.Error("Expected error for commit without transaction")
	}
}

func TestDBRollback_NoTransaction(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	err := runScriptExpectError(t, eng, `(db-rollback)`)

	if err == nil {
		t.Error("Expected error for rollback without transaction")
	}
}

func TestDBQuery_WrongArgs(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	err := runScriptExpectError(t, eng, `(db-query 123)`)

	if err == nil {
		t.Error("Expected error for non-string SQL")
	}
}

func TestDBQuery_NoArgs(t *testing.T) {
	storage := setupTestStorage(t)
	eng, _ := createEngine(t, storage)

	err := runScriptExpectError(t, eng, `(db-query)`)

	if err == nil {
		t.Error("Expected error for missing SQL argument")
	}
}
