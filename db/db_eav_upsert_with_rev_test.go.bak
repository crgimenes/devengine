package db

import "testing"

func TestEAVUpsertValueWithRevOptimisticLocking(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Product", "product", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(
		et.ID,
		"price", "Price", "",
		"REAL",
		false, false, false, false,
		"",
	)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Initial rev should be 1
	if rec.Rev != 1 {
		t.Fatalf("expected initial rev = 1, got %d", rec.Rev)
	}

	// Upsert value with correct rev (should increment to 2)
	price := 99.99
	newRev, err := s.UpsertEAVValueWithRev(rec.ID, attr.ID, 1, nil, nil, &price, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValueWithRev() error: %v", err)
	}

	if newRev != 2 {
		t.Errorf("expected new rev = 2, got %d", newRev)
	}

	// Verify value was set
	values, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}

	if values[0].VReal == nil || *values[0].VReal != 99.99 {
		t.Error("expected v_real = 99.99")
	}

	// Try to update with old rev (should fail with ErrConflict)
	newPrice := 149.99
	_, err = s.UpsertEAVValueWithRev(rec.ID, attr.ID, 1, nil, nil, &newPrice, nil, nil)
	if err != ErrConflict {
		t.Errorf("expected ErrConflict when using old rev, got %v", err)
	}

	// Update with correct rev (should increment to 3)
	newRev, err = s.UpsertEAVValueWithRev(rec.ID, attr.ID, 2, nil, nil, &newPrice, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValueWithRev() with rev 2 error: %v", err)
	}

	if newRev != 3 {
		t.Errorf("expected new rev = 3, got %d", newRev)
	}

	// Verify value was updated
	values, err = s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID() error: %v", err)
	}

	if values[0].VReal == nil || *values[0].VReal != 149.99 {
		t.Error("expected v_real = 149.99")
	}
}

func TestEAVUpsertValueWithRevTypeMismatch(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(
		et.ID,
		"count", "Count", "",
		"INT",
		false, false, false, false,
		"",
	)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord() error: %v", err)
	}

	// Try to set REAL value for INT attribute (should fail)
	val := 123.45
	_, err = s.UpsertEAVValueWithRev(rec.ID, attr.ID, 1, nil, nil, &val, nil, nil)
	if err == nil {
		t.Error("expected error for type mismatch")
	}
}
