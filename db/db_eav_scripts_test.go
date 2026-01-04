package db

import (
	"testing"
)

func TestExecutePosSaveEmptyScript(t *testing.T) {
	t.Parallel()

	entityType := &EAVEntityType{
		ID:      1,
		Name:    "Test",
		PreSave: "", // Empty script
	}

	values := EAVRecordValues{
		"field1": "value1",
		"field2": int64(42),
	}

	modifiedValues, userError, err := ExecutePreSaveScript(entityType, values)
	if err != nil {
		t.Fatalf("ExecutePreSaveScript() error: %v", err)
	}
	if userError != "" {
		t.Errorf("expected no userError, got %q", userError)
	}

	// Should return original values unchanged
	if modifiedValues["field1"] != "value1" {
		t.Errorf("expected field1 = 'value1', got %v", modifiedValues["field1"])
	}
	if modifiedValues["field2"] != int64(42) {
		t.Errorf("expected field2 = 42, got %v", modifiedValues["field2"])
	}
}

func TestExecutePosSaveModifiesValues(t *testing.T) {
	t.Parallel()

	entityType := &EAVEntityType{
		ID:      1,
		Name:    "Test",
		PreSave: `(set field1 (str-concat field1 " - modified"))`,
	}

	values := EAVRecordValues{
		"field1": "original",
	}

	modifiedValues, userError, err := ExecutePreSaveScript(entityType, values)
	if err != nil {
		t.Fatalf("ExecutePreSaveScript() error: %v", err)
	}
	if userError != "" {
		t.Errorf("expected no userError, got %q", userError)
	}

	// field1 should be modified by the script
	expected := "original - modified"
	if modifiedValues["field1"] != expected {
		t.Errorf("expected field1 = %q, got %v", expected, modifiedValues["field1"])
	}
}

func TestExecutePosSaveErrorVariable(t *testing.T) {
	t.Parallel()

	entityType := &EAVEntityType{
		ID:      1,
		Name:    "Test",
		PreSave: `(set error "Campo obrigatório não preenchido")`,
	}

	values := EAVRecordValues{
		"field1": "",
	}

	modifiedValues, userError, err := ExecutePreSaveScript(entityType, values)
	if err != nil {
		t.Fatalf("ExecutePreSaveScript() error: %v", err)
	}

	// Should return user error
	if userError != "Campo obrigatório não preenchido" {
		t.Errorf("expected userError = 'Campo obrigatório não preenchido', got %q", userError)
	}
	if modifiedValues != nil {
		t.Errorf("expected nil modifiedValues when userError is set, got %v", modifiedValues)
	}
}

func TestExecutePreSaveScriptError(t *testing.T) {
	t.Parallel()

	entityType := &EAVEntityType{
		ID:      1,
		Name:    "Test",
		PreSave: `(undefined-function arg)`, // Invalid script
	}

	values := EAVRecordValues{
		"field1": "value",
	}

	modifiedValues, userError, err := ExecutePreSaveScript(entityType, values)

	// Should return system error
	if err == nil {
		t.Fatal("expected system error for invalid script")
	}
	if userError != "" {
		t.Errorf("expected no userError, got %q", userError)
	}
	if modifiedValues != nil {
		t.Errorf("expected nil modifiedValues on error, got %v", modifiedValues)
	}
}

func TestExecutePosSaveTypedValues(t *testing.T) {
	t.Parallel()

	entityType := &EAVEntityType{
		ID:   1,
		Name: "Test",
		PreSave: `
			(set count (+ count 1))
			(set price (* price 1.1))
			(set active (not active))
		`,
	}

	values := EAVRecordValues{
		"count":  int64(10),
		"price":  100.0,
		"active": true,
	}

	modifiedValues, userError, err := ExecutePreSaveScript(entityType, values)
	if err != nil {
		t.Fatalf("ExecutePreSaveScript() error: %v", err)
	}
	if userError != "" {
		t.Errorf("expected no userError, got %q", userError)
	}

	// count should be 11
	if modifiedValues["count"] != int64(11) {
		t.Errorf("expected count = 11, got %v (type %T)", modifiedValues["count"], modifiedValues["count"])
	}

	// price should be 110.0 (allow small float precision error)
	if price, ok := modifiedValues["price"].(float64); !ok {
		t.Errorf("expected price to be float64, got %T", modifiedValues["price"])
	} else if price < 109.9 || price > 110.1 {
		t.Errorf("expected price ≈ 110.0, got %v", price)
	}

	// active should be false (negated)
	if modifiedValues["active"] != false {
		t.Errorf("expected active = false, got %v", modifiedValues["active"])
	}
}

func TestExecutePosSaveConditionalValidation(t *testing.T) {
	t.Parallel()

	// Validate that price must be positive
	entityType := &EAVEntityType{
		ID:   1,
		Name: "Test",
		PreSave: `
			(if (< price 0)
				(set error "Preço deve ser positivo"))
		`,
	}

	// Test with invalid price
	values := EAVRecordValues{
		"price": -50.0,
	}

	modifiedValues, userError, err := ExecutePreSaveScript(entityType, values)
	if err != nil {
		t.Fatalf("ExecutePreSaveScript() error: %v", err)
	}

	// Should have user error
	if userError != "Preço deve ser positivo" {
		t.Errorf("expected userError = 'Preço deve ser positivo', got %q", userError)
	}
	if modifiedValues != nil {
		t.Errorf("expected nil modifiedValues when validation fails, got %v", modifiedValues)
	}

	// Test with valid price
	values = EAVRecordValues{
		"price": 50.0,
	}

	modifiedValues, userError, err = ExecutePreSaveScript(entityType, values)
	if err != nil {
		t.Fatalf("ExecutePreSaveScript() error: %v", err)
	}
	if userError != "" {
		t.Errorf("expected no userError for valid price, got %q", userError)
	}
	if modifiedValues == nil {
		t.Error("expected modifiedValues for valid price")
	}
}
