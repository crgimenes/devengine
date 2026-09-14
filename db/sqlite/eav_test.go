package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/crgimenes/devengine/db"
)

// initTestDBWithEAVMigrations creates a test database and runs migrations up to EAV schema.
func initTestDBWithEAVMigrations(t *testing.T) *SQLite {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.db")

	s, err := NewWithPath(path)
	if err != nil {
		t.Fatalf("NewWithPath() error: %v", err)
	}

	// Run migrations on the isolated test database
	if err := RunMigrationOn(s); err != nil {
		s.Close()
		t.Fatalf("RunMigrationOn() error: %v", err)
	}

	return s
}

// ====================================================================
// Entity Type Tests
// ====================================================================

func TestEAVCreateEntityType(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Contact", "contact", "Manage contacts", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	if et.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if et.ReferenceID == "" {
		t.Error("expected non-empty reference_id")
	}
	if et.MachineName != "contact" {
		t.Errorf("expected machine_name 'contact', got %q", et.MachineName)
	}
	if et.Name != "Contact" {
		t.Errorf("expected name 'Contact', got %q", et.Name)
	}
	if et.Description != "Manage contacts" {
		t.Errorf("expected description 'Manage contacts', got %q", et.Description)
	}
}

func TestEAVGetEntityTypeByID(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	created, err := s.CreateEAVEntityType("Product", "product", "Product catalog", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	retrieved, err := s.GetEAVEntityTypeByID(created.ID)
	if err != nil {
		t.Fatalf("GetEAVEntityTypeByID() error: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, retrieved.ID)
	}
	if retrieved.ReferenceID != created.ReferenceID {
		t.Errorf("expected reference_id %s, got %s", created.ReferenceID, retrieved.ReferenceID)
	}
}

func TestEAVGetEntityTypeByRefID(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	created, err := s.CreateEAVEntityType("Order", "order", "Sales orders", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	retrieved, err := s.GetEAVEntityTypeByRefID(created.ReferenceID)
	if err != nil {
		t.Fatalf("GetEAVEntityTypeByRefID() error: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, retrieved.ID)
	}
}

func TestEAVGetEntityTypeByMachineName(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	created, err := s.CreateEAVEntityType("Invoice", "invoice", "Invoices", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	// Test case-insensitive lookup
	retrieved, err := s.GetEAVEntityTypeByMachineName("INVOICE")
	if err != nil {
		t.Fatalf("GetEAVEntityTypeByMachineName() error: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, retrieved.ID)
	}
}

func TestEAVListEntityTypes(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	// Create entity types in non-alphabetical order
	_, err := s.CreateEAVEntityType("Zebra", "zebra", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	_, err = s.CreateEAVEntityType("Alpha", "alpha", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	_, err = s.CreateEAVEntityType("Beta", "beta", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	list, err := s.ListEAVEntityTypes()
	if err != nil {
		t.Fatalf("ListEAVEntityTypes() error: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 entity types, got %d", len(list))
	}

	// Verify alphabetical order by machine_name
	expected := []string{"alpha", "beta", "zebra"}
	for i, et := range list {
		if et.MachineName != expected[i] {
			t.Errorf("expected machine_name[%d] = %q, got %q", i, expected[i], et.MachineName)
		}
	}
}

func TestEAVSoftDeleteEntityType(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	created, err := s.CreateEAVEntityType("Temp", "temp", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	// Soft delete
	if err := s.SoftDeleteEAVEntityType(created.ID); err != nil {
		t.Fatalf("SoftDeleteEAVEntityType() error: %v", err)
	}

	// Should not be found
	_, err = s.GetEAVEntityTypeByID(created.ID)
	if err != db.ErrNotFound {
		t.Errorf("expected db.ErrNotFound, got %v", err)
	}

	// Should not appear in list
	list, err := s.ListEAVEntityTypes()
	if err != nil {
		t.Fatalf("ListEAVEntityTypes() error: %v", err)
	}

	for _, et := range list {
		if et.ID == created.ID {
			t.Error("soft-deleted entity type should not appear in list")
		}
	}
}

func TestEAVUpdateEntityType(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	// Create entity type
	created, err := s.CreateEAVEntityType("Original Name", "original_name", "Original desc", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	// Update entity type
	updated, err := s.UpdateEAVEntityType(created.ID, "New Name", "New description", "(set error \"test\")", "")
	if err != nil {
		t.Fatalf("UpdateEAVEntityType() error: %v", err)
	}

	// Verify updated values
	if updated.Name != "New Name" {
		t.Errorf("expected Name = 'New Name', got %q", updated.Name)
	}
	if updated.Description != "New description" {
		t.Errorf("expected Description = 'New description', got %q", updated.Description)
	}
	if updated.PreSave != "(set error \"test\")" {
		t.Errorf("expected PosSave = '(set error \"test\")', got %q", updated.PreSave)
	}

	// machine_name should not change
	if updated.MachineName != "original_name" {
		t.Errorf("expected MachineName = 'original_name', got %q", updated.MachineName)
	}

	// Verify by fetching again
	fetched, err := s.GetEAVEntityTypeByID(updated.ID)
	if err != nil {
		t.Fatalf("GetEAVEntityTypeByID() error: %v", err)
	}
	if fetched.Name != "New Name" {
		t.Errorf("fetched Name = %q, expected 'New Name'", fetched.Name)
	}
	if fetched.PreSave != "(set error \"test\")" {
		t.Errorf("fetched PosSave = %q, expected '(set error \"test\")'", fetched.PreSave)
	}
}

// ====================================================================
// Attribute Tests
// ====================================================================

func TestEAVCreateAttribute(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Person", "person", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(
		et.ID,
		"full_name", "Full Name", "Enter your full name",
		"TEXT",
		true, false, false,
		nil,                     // maxLength
		false,                   // isComputed
		"",                      // computedExpr
		nil, nil, nil, nil, nil, // default values
	)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	if attr.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if attr.ReferenceID == "" {
		t.Error("expected non-empty reference_id")
	}
	if attr.MachineName != "full_name" {
		t.Errorf("expected machine_name 'full_name', got %q", attr.MachineName)
	}
	if attr.PrimitiveKind != "TEXT" {
		t.Errorf("expected primitive_kind 'TEXT', got %q", attr.PrimitiveKind)
	}
	if !attr.IsRequired {
		t.Error("expected is_required to be true")
	}
}

func TestEAVCreateAttributeInvalidKind(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	_, err = s.CreateEAVAttribute(
		et.ID,
		"bad_field", "Bad", "",
		"INVALID_KIND", // invalid
		false, false, false,
		nil,   // maxLength
		false, // isComputed
		"",    // computedExpr
		nil, nil, nil, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error for invalid primitive_kind")
	}
}

func TestEAVCreateAttributeAllKinds(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("AllTypes", "all_types", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	kinds := []string{"BOOL", "INT", "REAL", "TEXT", "DATETIME"}
	for _, kind := range kinds {
		_, err := s.CreateEAVAttribute(
			et.ID,
			"field_"+kind, "Field "+kind, "",
			kind,
			false, false, false,
			nil,   // maxLength
			false, // isComputed
			"",    // computedExpr
			nil, nil, nil, nil, nil,
		)
		if err != nil {
			t.Errorf("CreateEAVAttribute() for kind %s error: %v", kind, err)
		}
	}
}

func TestEAVListAttributesByEntityType(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Book", "book", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	// Create attributes in non-alphabetical order
	_, err = s.CreateEAVAttribute(et.ID, "title", "Title", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	_, err = s.CreateEAVAttribute(et.ID, "author", "Author", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	_, err = s.CreateEAVAttribute(et.ID, "year", "Year", "", "INT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	list, err := s.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil {
		t.Fatalf("ListEAVAttributesByEntityTypeID() error: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 attributes, got %d", len(list))
	}

	// Verify alphabetical order by machine_name
	expected := []string{"author", "title", "year"}
	for i, attr := range list {
		if attr.MachineName != expected[i] {
			t.Errorf("expected machine_name[%d] = %q, got %q", i, expected[i], attr.MachineName)
		}
	}
}

func TestEAVSoftDeleteAttribute(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "temp", "Temp", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	// Soft delete
	if err := s.SoftDeleteEAVAttribute(attr.ID); err != nil {
		t.Fatalf("SoftDeleteEAVAttribute() error: %v", err)
	}

	// Should not be found
	_, err = s.GetEAVAttributeByID(attr.ID)
	if err != db.ErrNotFound {
		t.Errorf("expected db.ErrNotFound, got %v", err)
	}

	// Should not appear in list
	list, err := s.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil {
		t.Fatalf("ListEAVAttributesByEntityTypeID() error: %v", err)
	}

	for _, a := range list {
		if a.ID == attr.ID {
			t.Error("soft-deleted attribute should not appear in list")
		}
	}
}

// ====================================================================
// Record Tests
// ====================================================================

func TestEAVCreateRecord(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Task", "task", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	if rec.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if rec.ReferenceID == "" {
		t.Error("expected non-empty reference_id")
	}
	if rec.Rev != 1 {
		t.Errorf("expected rev = 1, got %d", rec.Rev)
	}
	if rec.EntityTypeID != et.ID {
		t.Errorf("expected entity_type_id %d, got %d", et.ID, rec.EntityTypeID)
	}
}

func TestEAVGetRecordByRefID(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Note", "note", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	created, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	retrieved, err := s.GetEAVRecordByRefID(created.ReferenceID)
	if err != nil {
		t.Fatalf("GetEAVRecordByRefID() error: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, retrieved.ID)
	}
}

func TestEAVListRecordsPagination(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Item", "item", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	// Create 5 records
	for range 5 {
		_, err := s.CreateEAVRecord(et.ID)
		if err != nil {
			t.Fatalf("CreateEAVRecord() error: %v", err)
		}
		time.Sleep(2 * time.Millisecond) // ensure different timestamps
	}

	// Get first page (2 records)
	page1, total, err := s.ListEAVRecordsByEntityTypeID(et.ID, 2, 0)
	if err != nil {
		t.Fatalf("ListEAVRecordsByEntityTypeID() error: %v", err)
	}

	if total != 5 {
		t.Errorf("expected total = 5, got %d", total)
	}
	if len(page1) != 2 {
		t.Errorf("expected 2 records in page 1, got %d", len(page1))
	}

	// Get second page (2 records)
	page2, _, err := s.ListEAVRecordsByEntityTypeID(et.ID, 2, 2)
	if err != nil {
		t.Fatalf("ListEAVRecordsByEntityTypeID() error: %v", err)
	}

	if len(page2) != 2 {
		t.Errorf("expected 2 records in page 2, got %d", len(page2))
	}

	// Verify no overlap
	for _, r1 := range page1 {
		for _, r2 := range page2 {
			if r1.ID == r2.ID {
				t.Error("page 1 and page 2 should not have overlapping records")
			}
		}
	}
}

func TestEAVUpdateRecordRevOptimisticLocking(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Doc", "doc", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	if rec.Rev != 1 {
		t.Fatalf("expected initial rev = 1, got %d", rec.Rev)
	}

	// Update with correct rev
	newRev, err := s.UpdateEAVRecordRev(rec.ID, 1)
	if err != nil {
		t.Fatalf("UpdateEAVRecordRev() error: %v", err)
	}

	if newRev != 2 {
		t.Errorf("expected new rev = 2, got %d", newRev)
	}

	// Try to update with old rev (should fail)
	_, err = s.UpdateEAVRecordRev(rec.ID, 1)
	if err != db.ErrConflict {
		t.Errorf("expected db.ErrConflict, got %v", err)
	}

	// Update with correct rev
	newRev, err = s.UpdateEAVRecordRev(rec.ID, 2)
	if err != nil {
		t.Fatalf("UpdateEAVRecordRev() error: %v", err)
	}

	if newRev != 3 {
		t.Errorf("expected new rev = 3, got %d", newRev)
	}
}

func TestEAVSoftDeleteRecord(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Temp", "temp", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Soft delete
	if err := s.SoftDeleteEAVRecord(rec.ID); err != nil {
		t.Fatalf("SoftDeleteEAVRecord() error: %v", err)
	}

	// Should not be found
	_, err = s.GetEAVRecordByID(rec.ID)
	if err != db.ErrNotFound {
		t.Errorf("expected db.ErrNotFound, got %v", err)
	}

	// Should not appear in list
	list, _, err := s.ListEAVRecordsByEntityTypeID(et.ID, 100, 0)
	if err != nil {
		t.Fatalf("ListEAVRecordsByEntityTypeID() error: %v", err)
	}

	for _, r := range list {
		if r.ID == rec.ID {
			t.Error("soft-deleted record should not appear in list")
		}
	}
}

// ====================================================================
// Value Tests
// ====================================================================

func TestEAVUpsertValueBool(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Settings", "settings", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "enabled", "Enabled", "", "BOOL", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Insert bool value
	valBool := true
	err = s.UpsertEAVValue(rec.ID, attr.ID, &valBool, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Retrieve and verify
	values, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}

	if values[0].VBool == nil || *values[0].VBool != true {
		t.Error("expected v_bool = true")
	}
}

func TestEAVUpsertValueInt(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Counter", "counter", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "count", "Count", "", "INT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Insert int value
	valInt := int64(42)
	err = s.UpsertEAVValue(rec.ID, attr.ID, nil, &valInt, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Retrieve and verify
	values, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}

	if values[0].VInt == nil || *values[0].VInt != 42 {
		t.Error("expected v_int = 42")
	}
}

func TestEAVUpsertValueReal(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Measurement", "measurement", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "temperature", "Temperature", "", "REAL", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Insert real value
	valReal := 98.6
	err = s.UpsertEAVValue(rec.ID, attr.ID, nil, nil, &valReal, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Retrieve and verify
	values, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}

	if values[0].VReal == nil || *values[0].VReal != 98.6 {
		t.Error("expected v_real = 98.6")
	}
}

func TestEAVUpsertValueText(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Article", "article", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "title", "Title", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Insert text value
	valText := "Hello, World!"
	err = s.UpsertEAVValue(rec.ID, attr.ID, nil, nil, nil, &valText, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Retrieve and verify
	values, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}

	if values[0].VText == nil || *values[0].VText != "Hello, World!" {
		t.Error("expected v_text = 'Hello, World!'")
	}
}

func TestEAVUpsertValueDatetime(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Event", "event", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "scheduled_at", "Scheduled At", "", "DATETIME", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Insert datetime value (ISO-8601 UTC)
	valDatetime := "2024-12-18T10:00:00Z"
	err = s.UpsertEAVValue(rec.ID, attr.ID, nil, nil, nil, nil, &valDatetime)
	if err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Retrieve and verify
	values, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}

	if values[0].VDatetime == nil || *values[0].VDatetime != "2024-12-18T10:00:00Z" {
		t.Error("expected v_datetime = '2024-12-18T10:00:00Z'")
	}
}

func TestEAVUpsertValueTypeMismatch(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	// Create INT attribute
	attr, err := s.CreateEAVAttribute(et.ID, "number", "Number", "", "INT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Try to insert TEXT value into INT attribute (should fail)
	valText := "not a number"
	err = s.UpsertEAVValue(rec.ID, attr.ID, nil, nil, nil, &valText, nil)
	if err == nil {
		t.Fatal("expected error for type mismatch")
	}
}

func TestEAVUpsertValueMultipleValues(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "field", "Field", "", "INT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Try to set multiple values (should fail)
	valInt := int64(42)
	valText := "text"
	err = s.UpsertEAVValue(rec.ID, attr.ID, nil, &valInt, nil, &valText, nil)
	if err == nil {
		t.Fatal("expected error for multiple values")
	}
}

func TestEAVValueUpsertSemantics(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "version", "Version", "", "INT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Insert initial value
	val1 := int64(1)
	err = s.UpsertEAVValue(rec.ID, attr.ID, nil, &val1, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Update value
	val2 := int64(2)
	err = s.UpsertEAVValue(rec.ID, attr.ID, nil, &val2, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Retrieve and verify updated value
	values, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}

	if values[0].VInt == nil || *values[0].VInt != 2 {
		t.Error("expected v_int = 2 (updated value)")
	}
}

func TestEAVDeleteValue(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "field", "Field", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Insert value
	valText := "test"
	err = s.UpsertEAVValue(rec.ID, attr.ID, nil, nil, nil, &valText, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Delete value
	err = s.DeleteEAVValue(rec.ID, attr.ID)
	if err != nil {
		t.Fatalf("DeleteEAVValue() error: %v", err)
	}

	// Verify deleted
	values, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 0 {
		t.Errorf("expected 0 values after delete, got %d", len(values))
	}
}

func TestEAVGetValuesExcludesSoftDeletedAttributes(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr1, err := s.CreateEAVAttribute(et.ID, "field1", "Field 1", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	attr2, err := s.CreateEAVAttribute(et.ID, "field2", "Field 2", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Insert values for both attributes
	val1 := "value1"
	val2 := "value2"
	err = s.UpsertEAVValue(rec.ID, attr1.ID, nil, nil, nil, &val1, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}
	err = s.UpsertEAVValue(rec.ID, attr2.ID, nil, nil, nil, &val2, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Soft delete attr1
	err = s.SoftDeleteEAVAttribute(attr1.ID)
	if err != nil {
		t.Fatalf("SoftDeleteEAVAttribute() error: %v", err)
	}

	// Get values - should only return attr2's value
	values, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 1 {
		t.Fatalf("expected 1 value (attr2 only), got %d", len(values))
	}

	if values[0].AttributeID != attr2.ID {
		t.Error("expected only attr2's value to be returned")
	}
}

// ====================================================================
// Integration Tests
// ====================================================================

func TestEAVFullFlow(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	// 1. Create entity type
	et, err := s.CreateEAVEntityType("Person", "person", "Person records", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	// 2. Create attributes (TEXT + INT + DATETIME)
	attrName, err := s.CreateEAVAttribute(et.ID, "name", "Name", "", "TEXT", true, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(name) error: %v", err)
	}

	attrAge, err := s.CreateEAVAttribute(et.ID, "age", "Age", "", "INT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(age) error: %v", err)
	}

	attrBirthday, err := s.CreateEAVAttribute(et.ID, "birthday", "Birthday", "", "DATETIME", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(birthday) error: %v", err)
	}

	// 3. Create record
	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	if rec.Rev != 1 {
		t.Fatalf("expected rev=1, got %d", rec.Rev)
	}

	// 4. Upsert values (including datetime as ISO-8601)
	valName := "Alice"
	err = s.UpsertEAVValue(rec.ID, attrName.ID, nil, nil, nil, &valName, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue(name) error: %v", err)
	}

	valAge := int64(30)
	err = s.UpsertEAVValue(rec.ID, attrAge.ID, nil, &valAge, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue(age) error: %v", err)
	}

	valBirthday := "1994-12-18T00:00:00Z"
	err = s.UpsertEAVValue(rec.ID, attrBirthday.ID, nil, nil, nil, nil, &valBirthday)
	if err != nil {
		t.Fatalf("UpsertEAVValue(birthday) error: %v", err)
	}

	// 5. Get record by ref_id, verify rev=1
	retrieved, err := s.GetEAVRecordByRefID(rec.ReferenceID)
	if err != nil {
		t.Fatalf("GetEAVRecordByRefID() error: %v", err)
	}

	if retrieved.Rev != 1 {
		t.Errorf("expected rev=1, got %d", retrieved.Rev)
	}

	// 6. Load values and verify types
	values, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(values))
	}

	// Find each value by attribute ID
	var foundName, foundAge, foundBirthday bool
	for _, val := range values {
		switch val.AttributeID {
		case attrName.ID:
			if val.VText == nil || *val.VText != "Alice" {
				t.Error("expected name = 'Alice'")
			}
			foundName = true
		case attrAge.ID:
			if val.VInt == nil || *val.VInt != 30 {
				t.Error("expected age = 30")
			}
			foundAge = true
		case attrBirthday.ID:
			if val.VDatetime == nil || *val.VDatetime != "1994-12-18T00:00:00Z" {
				t.Error("expected birthday = '1994-12-18T00:00:00Z'")
			}
			foundBirthday = true
		}
	}

	if !foundName || !foundAge || !foundBirthday {
		t.Error("not all values were found")
	}

	// 7. Simulate concurrency: try update with wrong rev → expect db.ErrConflict
	_, err = s.UpdateEAVRecordRev(rec.ID, 999)
	if err != db.ErrConflict {
		t.Errorf("expected db.ErrConflict for wrong rev, got %v", err)
	}

	// 8. Update with correct rev → expect rev=2
	newRev, err := s.UpdateEAVRecordRev(rec.ID, 1)
	if err != nil {
		t.Fatalf("UpdateEAVRecordRev() error: %v", err)
	}

	if newRev != 2 {
		t.Errorf("expected rev=2, got %d", newRev)
	}

	// 9. Soft delete attribute and verify not in values list
	err = s.SoftDeleteEAVAttribute(attrBirthday.ID)
	if err != nil {
		t.Fatalf("SoftDeleteEAVAttribute() error: %v", err)
	}

	values, err = s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 2 {
		t.Errorf("expected 2 values after soft delete, got %d", len(values))
	}

	// 10. Soft delete record and verify not in list
	err = s.SoftDeleteEAVRecord(rec.ID)
	if err != nil {
		t.Fatalf("SoftDeleteEAVRecord() error: %v", err)
	}

	list, total, err := s.ListEAVRecordsByEntityTypeID(et.ID, 100, 0)
	if err != nil {
		t.Fatalf("ListEAVRecordsByEntityTypeID() error: %v", err)
	}

	if total != 0 || len(list) != 0 {
		t.Error("expected no records after soft delete")
	}
}

// ====================================================================
// New aggregation / lookup / tx helper tests
// ====================================================================

func TestSQLite_CountEAVRecords(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	for range 3 {
		_, err := s.CreateEAVRecord(et.ID)
		if err != nil {
			t.Fatalf("CreateEAVRecord() error: %v", err)
		}
	}

	n, err := s.CountEAVRecords(et.ID)
	if err != nil {
		t.Fatalf("CountEAVRecords() error: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

func TestSQLite_CountEAVRecords_ExcludesSoftDeleted(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	_, err = s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	if err := s.SoftDeleteEAVRecord(rec.ID); err != nil {
		t.Fatalf("SoftDeleteEAVRecord() error: %v", err)
	}

	n, err := s.CountEAVRecords(et.ID)
	if err != nil {
		t.Fatalf("CountEAVRecords() error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 (after soft delete), got %d", n)
	}
}

func TestSQLite_CountEAVRecordsWhere_Text(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "color", "Color", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec1, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}
	rec2, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	red := "red"
	blue := "blue"
	if err := s.UpsertEAVValue(rec1.ID, attr.ID, nil, nil, nil, &red, nil); err != nil {
		t.Fatalf("UpsertEAVValue(red) error: %v", err)
	}
	if err := s.UpsertEAVValue(rec2.ID, attr.ID, nil, nil, nil, &blue, nil); err != nil {
		t.Fatalf("UpsertEAVValue(blue) error: %v", err)
	}

	n, err := s.CountEAVRecordsWhere(et.ID, attr.ID, "TEXT", "red")
	if err != nil {
		t.Fatalf("CountEAVRecordsWhere() error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 record with 'red', got %d", n)
	}
}

func TestSQLite_CountEAVRecordsWhere_InvalidKind(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	_, err := s.CountEAVRecordsWhere(1, 1, "INVALID", nil)
	if err == nil {
		t.Fatal("expected error for invalid primitive kind")
	}
}

func TestSQLite_ListEAVRecordsByAttributeValue(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "status", "Status", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	// Create 2 records with "active", 1 with "inactive"
	for i := 0; i < 2; i++ {
		rec, err := s.CreateEAVRecord(et.ID)
		if err != nil {
			t.Fatalf("CreateEAVRecord() error: %v", err)
		}
		v := "active"
		if err := s.UpsertEAVValue(rec.ID, attr.ID, nil, nil, nil, &v, nil); err != nil {
			t.Fatalf("UpsertEAVValue() error: %v", err)
		}
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}
	v := "inactive"
	if err := s.UpsertEAVValue(rec.ID, attr.ID, nil, nil, nil, &v, nil); err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	results, err := s.ListEAVRecordsByAttributeValue(et.ID, attr.ID, "active")
	if err != nil {
		t.Fatalf("ListEAVRecordsByAttributeValue() error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 records with 'active', got %d", len(results))
	}
}

func TestSQLite_ListEAVRecordsByAttributeValue_ExcludesSoftDeleted(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "status", "Status", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec1, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}
	v := "active"
	if err := s.UpsertEAVValue(rec1.ID, attr.ID, nil, nil, nil, &v, nil); err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	rec2, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}
	if err := s.UpsertEAVValue(rec2.ID, attr.ID, nil, nil, nil, &v, nil); err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Soft delete rec1
	if err := s.SoftDeleteEAVRecord(rec1.ID); err != nil {
		t.Fatalf("SoftDeleteEAVRecord() error: %v", err)
	}

	results, err := s.ListEAVRecordsByAttributeValue(et.ID, attr.ID, "active")
	if err != nil {
		t.Fatalf("ListEAVRecordsByAttributeValue() error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 record (excluding soft-deleted), got %d", len(results))
	}
}

func TestTransaction_InsertEAVRecordWithRef(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	tx, err := s.BeginTransaction()
	if err != nil {
		t.Fatalf("BeginTransaction() error: %v", err)
	}
	defer tx.Rollback()

	recordID, err := tx.InsertEAVRecordWithRef("my-ref", et.ID, "draft")
	if err != nil {
		t.Fatalf("InsertEAVRecordWithRef() error: %v", err)
	}

	if recordID == 0 {
		t.Error("expected non-zero record ID")
	}

	// Verify via direct query within the same tx
	var status string
	var rev int
	var refID string
	err = tx.QueryRow(`SELECT status, rev, reference_id FROM eav_records WHERE id = ?`, recordID).Scan(&status, &rev, &refID)
	if err != nil {
		t.Fatalf("query within tx failed: %v", err)
	}
	if status != "draft" {
		t.Errorf("expected status 'draft', got %q", status)
	}
	if rev != 1 {
		t.Errorf("expected rev 1, got %d", rev)
	}
	if refID != "my-ref" {
		t.Errorf("expected refID 'my-ref', got %q", refID)
	}
}

func TestTransaction_UpsertEAVValue(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "score", "Score", "", "INT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	tx, err := s.BeginTransaction()
	if err != nil {
		t.Fatalf("BeginTransaction() error: %v", err)
	}
	defer tx.Rollback()

	vInt := int64(100)
	if err := tx.UpsertEAVValue(rec.ID, attr.ID, nil, &vInt, nil, nil, nil); err != nil {
		t.Fatalf("UpsertEAVValue() error: %v", err)
	}

	// Overwrite
	vInt2 := int64(200)
	if err := tx.UpsertEAVValue(rec.ID, attr.ID, nil, &vInt2, nil, nil, nil); err != nil {
		t.Fatalf("UpsertEAVValue() overwrite error: %v", err)
	}

	var got int64
	err = tx.QueryRow(`SELECT v_int FROM eav_values WHERE record_id = ? AND attribute_id = ?`, rec.ID, attr.ID).Scan(&got)
	if err != nil {
		t.Fatalf("query within tx failed: %v", err)
	}
	if got != 200 {
		t.Errorf("expected v_int 200 (overwritten), got %d", got)
	}
}

func TestTransaction_ActivateEAVRecord(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	tx, err := s.BeginTransaction()
	if err != nil {
		t.Fatalf("BeginTransaction() error: %v", err)
	}
	defer tx.Rollback()

	if err := tx.ActivateEAVRecord(rec.ID); err != nil {
		t.Fatalf("ActivateEAVRecord() error: %v", err)
	}

	var status string
	var rev int
	err = tx.QueryRow(`SELECT status, rev FROM eav_records WHERE id = ?`, rec.ID).Scan(&status, &rev)
	if err != nil {
		t.Fatalf("query within tx failed: %v", err)
	}
	if status != "active" {
		t.Errorf("expected status 'active', got %q", status)
	}
	if rev != 2 {
		t.Errorf("expected rev 2, got %d", rev)
	}
}
