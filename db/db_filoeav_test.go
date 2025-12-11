package db

import (
	"context"
	"testing"
	"time"

	"github.com/crgimenes/devengine/filo"
	"github.com/crgimenes/devengine/filoeav"
)

// TestFiloEAVGetValue tests the eav-get-value builtin end-to-end.
func TestFiloEAVGetValue(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	ctx := context.Background()

	// Create a test user
	if err := s.Exec(`INSERT INTO users (username, email, enabled) VALUES (?, ?, ?)`,
		"testuser", "test@example.com", 1); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Create a workspace
	ws, err := s.CreateEAVWorkspace("Test Workspace", "For testing filoeav")
	if err != nil {
		t.Fatalf("CreateEAVWorkspace: %v", err)
	}

	// Create Filo engine with EAV builtins
	eng := filo.NewEngine()
	filoeav.RegisterEAVBuiltins(eng, s, filoeav.Config{
		WorkspaceID: ws.ID,
		UserID:      1,
	})

	// Create a form with fields
	form, err := s.CreateEAVForm(ws.ID, 1, "products", "Products", "list")
	if err != nil {
		t.Fatalf("CreateEAVForm: %v", err)
	}

	// Create TEXT field
	_, err = s.CreateEAVField(form.ID, "name", "Product Name", 1, 12, "", false, false, "", "TEXT", "text_input", "{}", "", 0, false, false, true, 0, 12, true, 0, 12, true, 0, 12, false)
	if err != nil {
		t.Fatalf("CreateEAVField name: %v", err)
	}

	// Create INT field
	_, err = s.CreateEAVField(form.ID, "price", "Price", 2, 12, "", false, false, "", "INT", "number_input", "{}", "", 0, false, false, true, 0, 12, true, 0, 12, true, 0, 12, false)
	if err != nil {
		t.Fatalf("CreateEAVField price: %v", err)
	}

	// Create a record
	rec, err := s.CreateEAVRecord(form.ID, ws.ID, 1, "active", "")
	if err != nil {
		t.Fatalf("CreateEAVRecord: %v", err)
	}

	// Set values
	nameVal := "Widget"
	if err := s.SetEAVValue(rec.ID, 1, form.ID, nil, nil, nil, nil, &nameVal); err != nil {
		t.Fatalf("SetEAVValue name: %v", err)
	}

	priceVal := int64(100)
	if err := s.SetEAVValue(rec.ID, 2, form.ID, nil, nil, nil, &priceVal, nil); err != nil {
		t.Fatalf("SetEAVValue price: %v", err)
	}

	// Test eav-get-value for TEXT field
	cfg := filo.EvalConfig{StepLimit: 1000, RecursionLimit: 32, Timeout: 5 * time.Second}
	globals := map[string]filo.Value{
		"record-id": filo.VNum(float64(rec.ID)),
	}

	result, _, err := eng.RunScript(ctx, `(eav-get-value "products" record-id "name")`, globals, cfg)
	if err != nil {
		t.Fatalf("RunScript eav-get-value name: %v", err)
	}

	str, err := result.AsString()
	if err != nil {
		t.Fatalf("expected string result: %v", err)
	}
	if str != "Widget" {
		t.Errorf("expected 'Widget', got %q", str)
	}

	// Test eav-get-value for INT field
	result, _, err = eng.RunScript(ctx, `(eav-get-value "products" record-id "price")`, globals, cfg)
	if err != nil {
		t.Fatalf("RunScript eav-get-value price: %v", err)
	}

	num, err := result.AsNumber()
	if err != nil {
		t.Fatalf("expected number result: %v", err)
	}
	if num != 100 {
		t.Errorf("expected 100, got %v", num)
	}
}

// TestFiloEAVFindOne tests the eav-find-one builtin.
func TestFiloEAVFindOne(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	ctx := context.Background()

	if err := s.Exec(`INSERT INTO users (username, email, enabled) VALUES (?, ?, ?)`,
		"testuser", "test@example.com", 1); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	ws, err := s.CreateEAVWorkspace("Test Workspace", "")
	if err != nil {
		t.Fatalf("CreateEAVWorkspace: %v", err)
	}

	eng := filo.NewEngine()
	filoeav.RegisterEAVBuiltins(eng, s, filoeav.Config{WorkspaceID: ws.ID, UserID: 1})

	// Create form with field
	form, err := s.CreateEAVForm(ws.ID, 1, "users", "Users", "list")
	if err != nil {
		t.Fatalf("CreateEAVForm: %v", err)
	}

	_, err = s.CreateEAVField(form.ID, "email", "Email", 1, 12, "", false, false, "", "TEXT", "text_input", "{}", "", 0, false, false, true, 0, 12, true, 0, 12, true, 0, 12, false)
	if err != nil {
		t.Fatalf("CreateEAVField: %v", err)
	}

	// Create records
	rec1, _ := s.CreateEAVRecord(form.ID, ws.ID, 1, "active", "")
	rec2, _ := s.CreateEAVRecord(form.ID, ws.ID, 1, "active", "")

	email1 := "alice@example.com"
	email2 := "bob@example.com"
	s.SetEAVValue(rec1.ID, 1, form.ID, nil, nil, nil, nil, &email1)
	s.SetEAVValue(rec2.ID, 1, form.ID, nil, nil, nil, nil, &email2)

	cfg := filo.EvalConfig{StepLimit: 1000, RecursionLimit: 32, Timeout: 5 * time.Second}

	// Find existing record
	result, _, err := eng.RunScript(ctx, `(eav-find-one "users" "email" "=" "bob@example.com")`, nil, cfg)
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}

	num, err := result.AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	if int64(num) != rec2.ID {
		t.Errorf("expected record ID %d, got %v", rec2.ID, num)
	}

	// Find non-existent record
	result, _, err = eng.RunScript(ctx, `(eav-find-one "users" "email" "=" "nobody@example.com")`, nil, cfg)
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}

	b, err := result.AsBool()
	if err != nil {
		t.Fatalf("expected bool: %v", err)
	}
	if b != false {
		t.Error("expected #f for not found")
	}
}

// TestFiloEAVSum tests the eav-sum builtin.
func TestFiloEAVSum(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	ctx := context.Background()

	s.Exec(`INSERT INTO users (username, email, enabled) VALUES (?, ?, ?)`, "testuser", "test@example.com", 1)
	ws, _ := s.CreateEAVWorkspace("Test Workspace", "")

	eng := filo.NewEngine()
	filoeav.RegisterEAVBuiltins(eng, s, filoeav.Config{WorkspaceID: ws.ID, UserID: 1})

	// Create form with numeric field
	form, _ := s.CreateEAVForm(ws.ID, 1, "orders", "Orders", "list")
	_, _ = s.CreateEAVField(form.ID, "amount", "Amount", 1, 12, "", false, false, "", "FLOAT", "decimal_input", "{}", "", 0, false, false, true, 0, 12, true, 0, 12, true, 0, 12, false)

	// Create records with values
	amounts := []float64{100.50, 200.25, 50.75}
	for _, amt := range amounts {
		rec, _ := s.CreateEAVRecord(form.ID, ws.ID, 1, "active", "")
		s.SetEAVValue(rec.ID, 1, form.ID, nil, nil, &amt, nil, nil)
	}

	cfg := filo.EvalConfig{StepLimit: 1000, RecursionLimit: 32, Timeout: 5 * time.Second}

	// Test SUM
	result, _, err := eng.RunScript(ctx, `(eav-sum "orders" "amount" (list))`, nil, cfg)
	if err != nil {
		t.Fatalf("RunScript eav-sum: %v", err)
	}

	sum, _ := result.AsNumber()
	expected := 100.50 + 200.25 + 50.75
	if sum != expected {
		t.Errorf("expected sum %.2f, got %.2f", expected, sum)
	}
}

// TestFiloEAVCount tests the eav-count builtin.
func TestFiloEAVCount(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	ctx := context.Background()

	s.Exec(`INSERT INTO users (username, email, enabled) VALUES (?, ?, ?)`, "testuser", "test@example.com", 1)
	ws, _ := s.CreateEAVWorkspace("Test Workspace", "")

	eng := filo.NewEngine()
	filoeav.RegisterEAVBuiltins(eng, s, filoeav.Config{WorkspaceID: ws.ID, UserID: 1})

	form, _ := s.CreateEAVForm(ws.ID, 1, "items", "Items", "list")
	_, _ = s.CreateEAVField(form.ID, "name", "Name", 1, 12, "", false, false, "", "TEXT", "text_input", "{}", "", 0, false, false, true, 0, 12, true, 0, 12, true, 0, 12, false)

	// Create 5 records
	for i := 0; i < 5; i++ {
		s.CreateEAVRecord(form.ID, ws.ID, 1, "active", "")
	}

	cfg := filo.EvalConfig{StepLimit: 1000, RecursionLimit: 32, Timeout: 5 * time.Second}

	// Count all records (no field needed)
	result, _, err := eng.RunScript(ctx, `(eav-count "items" "" (list))`, nil, cfg)
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}

	count, _ := result.AsNumber()
	if count != 5 {
		t.Errorf("expected count 5, got %.0f", count)
	}
}

// TestFiloEAVAggregateWithFilters tests aggregation with filter conditions.
func TestFiloEAVAggregateWithFilters(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	ctx := context.Background()

	s.Exec(`INSERT INTO users (username, email, enabled) VALUES (?, ?, ?)`, "testuser", "test@example.com", 1)
	ws, _ := s.CreateEAVWorkspace("Test Workspace", "")

	eng := filo.NewEngine()
	filoeav.RegisterEAVBuiltins(eng, s, filoeav.Config{WorkspaceID: ws.ID, UserID: 1})

	form, _ := s.CreateEAVForm(ws.ID, 1, "sales", "Sales", "list")
	_, _ = s.CreateEAVField(form.ID, "category", "Category", 1, 12, "", false, false, "", "TEXT", "text_input", "{}", "", 0, false, false, true, 0, 12, true, 0, 12, true, 0, 12, false)
	_, _ = s.CreateEAVField(form.ID, "amount", "Amount", 2, 12, "", false, false, "", "INT", "number_input", "{}", "", 0, false, false, true, 0, 12, true, 0, 12, true, 0, 12, false)

	// Create sales records
	data := []struct {
		category string
		amount   int64
	}{
		{"electronics", 100},
		{"electronics", 200},
		{"clothing", 50},
		{"clothing", 75},
	}

	for _, d := range data {
		rec, _ := s.CreateEAVRecord(form.ID, ws.ID, 1, "active", "")
		s.SetEAVValue(rec.ID, 1, form.ID, nil, nil, nil, nil, &d.category)
		s.SetEAVValue(rec.ID, 2, form.ID, nil, nil, nil, &d.amount, nil)
	}

	cfg := filo.EvalConfig{StepLimit: 1000, RecursionLimit: 32, Timeout: 5 * time.Second}

	// Sum electronics only
	script := `(eav-sum "sales" "amount" (list (values "category" "=" "electronics")))`
	result, _, err := eng.RunScript(ctx, script, nil, cfg)
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}

	sum, _ := result.AsNumber()
	if sum != 300 { // 100 + 200
		t.Errorf("expected 300, got %.0f", sum)
	}
}

// TestFiloEAVBuiltinErrors tests error handling in EAV builtins.
func TestFiloEAVBuiltinErrors(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	s.Exec(`INSERT INTO users (username, email, enabled) VALUES (?, ?, ?)`, "testuser", "test@example.com", 1)
	ws, _ := s.CreateEAVWorkspace("Test Workspace", "")

	eng := filo.NewEngine()
	filoeav.RegisterEAVBuiltins(eng, s, filoeav.Config{WorkspaceID: ws.ID, UserID: 1})

	ctx := context.Background()
	cfg := filo.EvalConfig{StepLimit: 1000, RecursionLimit: 32, Timeout: 5 * time.Second}

	// Test wrong number of arguments
	_, _, err := eng.RunScript(ctx, `(eav-get-value "form")`, nil, cfg)
	if err == nil {
		t.Error("expected error for wrong argument count")
	}

	// Test invalid operator
	_, _, err = eng.RunScript(ctx, `(eav-find-one "form" "field" "LIKE" "value")`, nil, cfg)
	if err == nil {
		t.Error("expected error for invalid operator")
	}

	// Test form not found
	_, _, err = eng.RunScript(ctx, `(eav-get-value "nonexistent" 1 "field")`, nil, cfg)
	if err == nil {
		t.Error("expected error for nonexistent form")
	}
}
