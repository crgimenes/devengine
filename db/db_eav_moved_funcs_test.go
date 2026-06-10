package db

import (
	"testing"
)

// ====================================================================
// Value conversion helpers (UnwrapEAVValue / FormatEAVValue)
// ====================================================================

func TestUnwrapEAVValue_Bool(t *testing.T) {
	t.Parallel()
	v := true
	got := UnwrapEAVValue("BOOL", EAVValue{VBool: &v})
	if b, ok := got.(bool); !ok || b != true {
		t.Errorf("expected true, got %#v", got)
	}
}

func TestUnwrapEAVValue_Int(t *testing.T) {
	t.Parallel()
	v := int64(42)
	got := UnwrapEAVValue("INT", EAVValue{VInt: &v})
	if i, ok := got.(int64); !ok || i != 42 {
		t.Errorf("expected 42, got %#v", got)
	}
}

func TestUnwrapEAVValue_Real(t *testing.T) {
	t.Parallel()
	v := 3.14
	got := UnwrapEAVValue("REAL", EAVValue{VReal: &v})
	if f, ok := got.(float64); !ok || f != 3.14 {
		t.Errorf("expected 3.14, got %#v", got)
	}
}

func TestUnwrapEAVValue_Text(t *testing.T) {
	t.Parallel()
	v := "hello"
	got := UnwrapEAVValue("TEXT", EAVValue{VText: &v})
	if s, ok := got.(string); !ok || s != "hello" {
		t.Errorf("expected 'hello', got %#v", got)
	}
}

func TestUnwrapEAVValue_Datetime(t *testing.T) {
	t.Parallel()
	v := "2025-01-01T00:00:00Z"
	got := UnwrapEAVValue("DATETIME", EAVValue{VDatetime: &v})
	if s, ok := got.(string); !ok || s != "2025-01-01T00:00:00Z" {
		t.Errorf("expected datetime string, got %#v", got)
	}
}

func TestUnwrapEAVValue_NilWhenColumnIsNil(t *testing.T) {
	t.Parallel()
	got := UnwrapEAVValue("BOOL", EAVValue{VBool: nil})
	if got != nil {
		t.Errorf("expected nil when column is nil, got %#v", got)
	}
}

func TestUnwrapEAVValue_NilWhenUnknownKind(t *testing.T) {
	t.Parallel()
	v := int64(1)
	got := UnwrapEAVValue("UNKNOWN", EAVValue{VInt: &v})
	if got != nil {
		t.Errorf("expected nil for unknown kind, got %#v", got)
	}
}

func TestFormatEAVValue_Text(t *testing.T) {
	t.Parallel()
	v := "Alice"
	values := []EAVValue{{AttributeID: 1, VText: &v}}
	got := FormatEAVValue("TEXT", values, 1, "fallback")
	if got != "Alice" {
		t.Errorf("expected 'Alice', got %q", got)
	}
}

func TestFormatEAVValue_Int(t *testing.T) {
	t.Parallel()
	v := int64(99)
	values := []EAVValue{{AttributeID: 2, VInt: &v}}
	got := FormatEAVValue("INT", values, 2, "-")
	if got != "99" {
		t.Errorf("expected '99', got %q", got)
	}
}

func TestFormatEAVValue_Real(t *testing.T) {
	t.Parallel()
	v := 2.5
	values := []EAVValue{{AttributeID: 3, VReal: &v}}
	got := FormatEAVValue("REAL", values, 3, "-")
	if got != "2.5" {
		t.Errorf("expected '2.5', got %q", got)
	}
}

func TestFormatEAVValue_BoolTrue(t *testing.T) {
	t.Parallel()
	v := true
	values := []EAVValue{{AttributeID: 4, VBool: &v}}
	got := FormatEAVValue("BOOL", values, 4, "-")
	if got != "true" {
		t.Errorf("expected 'true', got %q", got)
	}
}

func TestFormatEAVValue_BoolFalse(t *testing.T) {
	t.Parallel()
	v := false
	values := []EAVValue{{AttributeID: 5, VBool: &v}}
	got := FormatEAVValue("BOOL", values, 5, "-")
	if got != "false" {
		t.Errorf("expected 'false', got %q", got)
	}
}

func TestFormatEAVValue_Datetime(t *testing.T) {
	t.Parallel()
	v := "2024-06-15T12:00:00Z"
	values := []EAVValue{{AttributeID: 6, VDatetime: &v}}
	got := FormatEAVValue("DATETIME", values, 6, "-")
	if got != "2024-06-15T12:00:00Z" {
		t.Errorf("expected datetime string, got %q", got)
	}
}

func TestFormatEAVValue_FallbackWhenNoMatch(t *testing.T) {
	t.Parallel()
	v := "irrelevant"
	values := []EAVValue{{AttributeID: 1, VText: &v}}
	got := FormatEAVValue("TEXT", values, 999, "fallback")
	if got != "fallback" {
		t.Errorf("expected 'fallback', got %q", got)
	}
}

func TestFormatEAVValue_FallbackWhenColumnIsNil(t *testing.T) {
	t.Parallel()
	values := []EAVValue{{AttributeID: 1, VText: nil}}
	got := FormatEAVValue("TEXT", values, 1, "fallback")
	if got != "fallback" {
		t.Errorf("expected 'fallback', got %q", got)
	}
}

func TestFormatEAVValue_SkipsNonMatchingAttributeIDs(t *testing.T) {
	t.Parallel()
	v := "target"
	values := []EAVValue{
		{AttributeID: 1, VText: strPtr("other")},
		{AttributeID: 2, VText: &v},
	}
	got := FormatEAVValue("TEXT", values, 2, "-")
	if got != "target" {
		t.Errorf("expected 'target', got %q", got)
	}
}

// ====================================================================
// Transaction-level helpers
// ====================================================================

func TestTransaction_CreateEAVRecordInTx(t *testing.T) {
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

	recordID, refID, err := tx.CreateEAVRecordInTx("my-ref", et.ID, "draft")
	if err != nil {
		t.Fatalf("CreateEAVRecordInTx() error: %v", err)
	}

	if recordID == 0 {
		t.Error("expected non-zero record ID")
	}
	if refID != "my-ref" {
		t.Errorf("expected refID 'my-ref', got %q", refID)
	}

	// Verify via direct query within the same tx
	var status string
	var rev int
	err = tx.QueryRow(`SELECT status, rev FROM eav_records WHERE id = ?`, recordID).Scan(&status, &rev)
	if err != nil {
		t.Fatalf("query within tx failed: %v", err)
	}
	if status != "draft" {
		t.Errorf("expected status 'draft', got %q", status)
	}
	if rev != 1 {
		t.Errorf("expected rev 1, got %d", rev)
	}
}

func TestTransaction_UpdateEAVRecordRevInTx(t *testing.T) {
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
	if rec.Rev != 1 {
		t.Fatalf("expected rev 1, got %d", rec.Rev)
	}

	tx, err := s.BeginTransaction()
	if err != nil {
		t.Fatalf("BeginTransaction() error: %v", err)
	}
	defer tx.Rollback()

	if err := tx.UpdateEAVRecordRevInTx(rec.ID); err != nil {
		t.Fatalf("UpdateEAVRecordRevInTx() error: %v", err)
	}

	var rev int
	err = tx.QueryRow(`SELECT rev FROM eav_records WHERE id = ?`, rec.ID).Scan(&rev)
	if err != nil {
		t.Fatalf("query within tx failed: %v", err)
	}
	if rev != 2 {
		t.Errorf("expected rev 2, got %d", rev)
	}
}

func TestTransaction_ActivateEAVRecordInTx(t *testing.T) {
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

	if err := tx.ActivateEAVRecordInTx(rec.ID); err != nil {
		t.Fatalf("ActivateEAVRecordInTx() error: %v", err)
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

func TestTransaction_UpsertEAVValueInTx(t *testing.T) {
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

	val := int64(100)
	if err := tx.UpsertEAVValueInTx(rec.ID, attr.ID, nil, &val, nil, nil, nil); err != nil {
		t.Fatalf("UpsertEAVValueInTx() error: %v", err)
	}

	var vInt int64
	err = tx.QueryRow(`SELECT v_int FROM eav_values WHERE record_id = ? AND attribute_id = ?`, rec.ID, attr.ID).Scan(&vInt)
	if err != nil {
		t.Fatalf("query within tx failed: %v", err)
	}
	if vInt != 100 {
		t.Errorf("expected v_int 100, got %d", vInt)
	}
}

func TestTransaction_UpsertEAVValueInTx_Overwrite(t *testing.T) {
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

	val1 := int64(1)
	if err := tx.UpsertEAVValueInTx(rec.ID, attr.ID, nil, &val1, nil, nil, nil); err != nil {
		t.Fatalf("first UpsertEAVValueInTx() error: %v", err)
	}

	val2 := int64(2)
	if err := tx.UpsertEAVValueInTx(rec.ID, attr.ID, nil, &val2, nil, nil, nil); err != nil {
		t.Fatalf("second UpsertEAVValueInTx() error: %v", err)
	}

	var vInt int64
	err = tx.QueryRow(`SELECT v_int FROM eav_values WHERE record_id = ? AND attribute_id = ?`, rec.ID, attr.ID).Scan(&vInt)
	if err != nil {
		t.Fatalf("query within tx failed: %v", err)
	}
	if vInt != 2 {
		t.Errorf("expected v_int 2 (overwritten), got %d", vInt)
	}
}

func TestTransaction_UpsertEAVValueInTx_EachKind(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("AllKinds", "all_kinds", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	kinds := []struct {
		name string
		kind string
	}{
		{"bool_attr", "BOOL"},
		{"int_attr", "INT"},
		{"real_attr", "REAL"},
		{"text_attr", "TEXT"},
		{"dt_attr", "DATETIME"},
	}

	var attrIDs []int64
	for _, k := range kinds {
		a, err := s.CreateEAVAttribute(et.ID, k.name, k.name, "", k.kind, false, false, false, nil, false, "", nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateEAVAttribute(%s) error: %v", k.name, err)
		}
		attrIDs = append(attrIDs, a.ID)
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

	vBool := true
	vInt := int64(7)
	vReal := 1.5
	vText := "hi"
	vDt := "2025-03-01T00:00:00Z"

	if err := tx.UpsertEAVValueInTx(rec.ID, attrIDs[0], &vBool, nil, nil, nil, nil); err != nil {
		t.Errorf("UpsertEAVValueInTx(bool) error: %v", err)
	}
	if err := tx.UpsertEAVValueInTx(rec.ID, attrIDs[1], nil, &vInt, nil, nil, nil); err != nil {
		t.Errorf("UpsertEAVValueInTx(int) error: %v", err)
	}
	if err := tx.UpsertEAVValueInTx(rec.ID, attrIDs[2], nil, nil, &vReal, nil, nil); err != nil {
		t.Errorf("UpsertEAVValueInTx(real) error: %v", err)
	}
	if err := tx.UpsertEAVValueInTx(rec.ID, attrIDs[3], nil, nil, nil, &vText, nil); err != nil {
		t.Errorf("UpsertEAVValueInTx(text) error: %v", err)
	}
	if err := tx.UpsertEAVValueInTx(rec.ID, attrIDs[4], nil, nil, nil, nil, &vDt); err != nil {
		t.Errorf("UpsertEAVValueInTx(datetime) error: %v", err)
	}

	// Verify all 5 values
	rows, err := tx.Query(`SELECT attribute_id, v_bool, v_int, v_real, v_text, v_datetime FROM eav_values WHERE record_id = ?`, rec.ID)
	if err != nil {
		t.Fatalf("query within tx failed: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var aid int64
		var vb, vi, vr, vt, vd any
		if err := rows.Scan(&aid, &vb, &vi, &vr, &vt, &vd); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		count++
	}
	if count != 5 {
		t.Errorf("expected 5 values, got %d", count)
	}
}

func TestTransaction_SaveEAVValuesTx(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attrA, err := s.CreateEAVAttribute(et.ID, "name", "Name", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(name) error: %v", err)
	}

	attrB, err := s.CreateEAVAttribute(et.ID, "age", "Age", "", "INT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(age) error: %v", err)
	}

	computed, err := s.CreateEAVAttribute(et.ID, "full", "Full", "", "TEXT", false, false, false, nil, true, "(concat name age)", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(computed) error: %v", err)
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

	values := EAVRecordValues{
		"name": "Alice",
		"age":  int64(30),
		"full": "should be skipped",
	}

	attributes := []EAVAttribute{*attrA, *attrB, *computed}

	if err := tx.SaveEAVValuesTx(rec.ID, attributes, values); err != nil {
		t.Fatalf("SaveEAVValuesTx() error: %v", err)
	}

	var name string
	err = tx.QueryRow(`SELECT v_text FROM eav_values WHERE record_id = ? AND attribute_id = ?`, rec.ID, attrA.ID).Scan(&name)
	if err != nil {
		t.Fatalf("query name failed: %v", err)
	}
	if name != "Alice" {
		t.Errorf("expected name 'Alice', got %q", name)
	}

	var age int64
	err = tx.QueryRow(`SELECT v_int FROM eav_values WHERE record_id = ? AND attribute_id = ?`, rec.ID, attrB.ID).Scan(&age)
	if err != nil {
		t.Fatalf("query age failed: %v", err)
	}
	if age != 30 {
		t.Errorf("expected age 30, got %d", age)
	}

	// Computed attribute should NOT have a value saved
	var computedCount int
	err = tx.QueryRow(`SELECT COUNT(*) FROM eav_values WHERE record_id = ? AND attribute_id = ?`, rec.ID, computed.ID).Scan(&computedCount)
	if err != nil {
		t.Fatalf("query computed count failed: %v", err)
	}
	if computedCount != 0 {
		t.Errorf("expected 0 values for computed attribute, got %d", computedCount)
	}
}

// ====================================================================
// Aggregation / lookup helpers
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

func TestSQLite_CountEAVRecordsWhere_AllKinds(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	kinds := []struct {
		name  string
		kind  string
		value any
		setFn func(recID, attrID int64, val any) error
	}{
		{"b", "BOOL", true, func(recID, attrID int64, val any) error {
			v := val.(bool)
			return s.UpsertEAVValue(recID, attrID, &v, nil, nil, nil, nil)
		}},
		{"i", "INT", int64(5), func(recID, attrID int64, val any) error {
			v := val.(int64)
			return s.UpsertEAVValue(recID, attrID, nil, &v, nil, nil, nil)
		}},
		{"r", "REAL", 1.5, func(recID, attrID int64, val any) error {
			v := val.(float64)
			return s.UpsertEAVValue(recID, attrID, nil, nil, &v, nil, nil)
		}},
		{"t", "TEXT", "x", func(recID, attrID int64, val any) error {
			v := val.(string)
			return s.UpsertEAVValue(recID, attrID, nil, nil, nil, &v, nil)
		}},
		{"d", "DATETIME", "2025-01-01T00:00:00Z", func(recID, attrID int64, val any) error {
			v := val.(string)
			return s.UpsertEAVValue(recID, attrID, nil, nil, nil, nil, &v)
		}},
	}

	for _, k := range kinds {
		a, err := s.CreateEAVAttribute(et.ID, k.name, k.name, "", k.kind, false, false, false, nil, false, "", nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateEAVAttribute(%s) error: %v", k.name, err)
		}

		rec, err := s.CreateEAVRecord(et.ID)
		if err != nil {
			t.Fatalf("CreateEAVRecord() error: %v", err)
		}

		if err := k.setFn(rec.ID, a.ID, k.value); err != nil {
			t.Fatalf("set value for %s error: %v", k.name, err)
		}

		n, err := s.CountEAVRecordsWhere(et.ID, a.ID, k.kind, k.value)
		if err != nil {
			t.Fatalf("CountEAVRecordsWhere(%s) error: %v", k.kind, err)
		}
		if n != 1 {
			t.Errorf("expected 1 for %s (kind=%s), got %d", k.name, k.kind, n)
		}
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

// ====================================================================
// Lookup helpers
// ====================================================================

func TestSQLite_LookupEAVEntityTypeAndAttribute_Found(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, err := s.CreateEAVEntityType("Product", "product", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	attr, err := s.CreateEAVAttribute(et.ID, "price", "Price", "", "INT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute() error: %v", err)
	}

	retET, retAttr, err := s.LookupEAVEntityTypeAndAttribute("product", "price")
	if err != nil {
		t.Fatalf("LookupEAVEntityTypeAndAttribute() error: %v", err)
	}
	if retET == nil {
		t.Fatal("expected entity type, got nil")
	}
	if retET.ID != et.ID {
		t.Errorf("expected entity type ID %d, got %d", et.ID, retET.ID)
	}
	if retAttr == nil {
		t.Fatal("expected attribute, got nil")
	}
	if retAttr.ID != attr.ID {
		t.Errorf("expected attribute ID %d, got %d", attr.ID, retAttr.ID)
	}
}

func TestSQLite_LookupEAVEntityTypeAndAttribute_EntityNotFound(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	et, attr, err := s.LookupEAVEntityTypeAndAttribute("nonexistent", "anything")
	if err != nil {
		t.Fatalf("LookupEAVEntityTypeAndAttribute() error: %v", err)
	}
	if et != nil {
		t.Error("expected nil entity type")
	}
	if attr != nil {
		t.Error("expected nil attribute")
	}
}

func TestSQLite_LookupEAVEntityTypeAndAttribute_AttributeNotFound(t *testing.T) {
	t.Parallel()

	s := initTestDBWithEAVMigrations(t)
	defer s.Close()

	_, err := s.CreateEAVEntityType("Test", "test", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType() error: %v", err)
	}

	et, attr, err := s.LookupEAVEntityTypeAndAttribute("test", "nosuchattr")
	if err != nil {
		t.Fatalf("LookupEAVEntityTypeAndAttribute() error: %v", err)
	}
	if et == nil {
		t.Fatal("expected entity type, got nil")
	}
	if attr != nil {
		t.Error("expected nil attribute when attr does not exist")
	}
}

func strPtr(s string) *string { return &s }
