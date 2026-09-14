package db

import (
	"errors"
	"testing"
)

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
		{AttributeID: 1, VText: new("other")},
		{AttributeID: 2, VText: &v},
	}
	got := FormatEAVValue("TEXT", values, 2, "-")
	if got != "target" {
		t.Errorf("expected 'target', got %q", got)
	}
}

type lookupStub struct {
	et      *EAVEntityType
	etErr   error
	attrs   []EAVAttribute
	attrErr error
}

func (s lookupStub) GetEAVEntityTypeByMachineName(string) (*EAVEntityType, error) {
	return s.et, s.etErr
}

func (s lookupStub) ListEAVAttributesByEntityTypeID(int64) ([]EAVAttribute, error) {
	return s.attrs, s.attrErr
}

func TestEAVValueColumn(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]string{
		"BOOL":     "v_bool",
		"INT":      "v_int",
		"REAL":     "v_real",
		"TEXT":     "v_text",
		"DATETIME": "v_datetime",
	} {
		got, err := EAVValueColumn(kind)
		if err != nil {
			t.Errorf("EAVValueColumn(%q) error: %v", kind, err)
		}
		if got != want {
			t.Errorf("EAVValueColumn(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestEAVValueColumn_UnknownKind(t *testing.T) {
	t.Parallel()
	_, err := EAVValueColumn("JSON")
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("error = %v, want ErrInvalidValue", err)
	}
}

func TestLookupEAVEntityTypeAndAttribute(t *testing.T) {
	t.Parallel()
	s := lookupStub{
		et: &EAVEntityType{ID: 7, MachineName: "product"},
		attrs: []EAVAttribute{
			{ID: 1, MachineName: "sku", PrimitiveKind: "TEXT"},
			{ID: 2, MachineName: "price", PrimitiveKind: "REAL"},
		},
	}
	et, attr, err := LookupEAVEntityTypeAndAttribute(s, "product", "price")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if et == nil || et.ID != 7 {
		t.Fatalf("entity type = %v, want ID 7", et)
	}
	if attr == nil || attr.ID != 2 {
		t.Fatalf("attribute = %v, want ID 2", attr)
	}
}

// A missing entity is not an error: the Filo builtins treat it as "no match"
// and return an empty value rather than aborting the script.
func TestLookupEAVEntityTypeAndAttribute_EntityNotFound(t *testing.T) {
	t.Parallel()
	for name, s := range map[string]lookupStub{
		"ErrNotFound": {etErr: ErrNotFound},
		"nil entity":  {et: nil},
	} {
		et, attr, err := LookupEAVEntityTypeAndAttribute(s, "ghost", "sku")
		if err != nil || et != nil || attr != nil {
			t.Errorf("%s: got (%v, %v, %v), want all nil", name, et, attr, err)
		}
	}
}

// An existing entity whose attribute is missing still returns the entity, so
// the caller can tell "no such entity" from "no such attribute".
func TestLookupEAVEntityTypeAndAttribute_AttributeNotFound(t *testing.T) {
	t.Parallel()
	s := lookupStub{
		et:    &EAVEntityType{ID: 7, MachineName: "product"},
		attrs: []EAVAttribute{{ID: 1, MachineName: "sku"}},
	}
	et, attr, err := LookupEAVEntityTypeAndAttribute(s, "product", "missing")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if et == nil || et.ID != 7 {
		t.Fatalf("entity type = %v, want ID 7", et)
	}
	if attr != nil {
		t.Fatalf("attribute = %v, want nil", attr)
	}
}

func TestLookupEAVEntityTypeAndAttribute_StorageErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")

	_, _, err := LookupEAVEntityTypeAndAttribute(lookupStub{etErr: boom}, "product", "sku")
	if !errors.Is(err, boom) {
		t.Errorf("entity lookup error = %v, want boom", err)
	}

	s := lookupStub{et: &EAVEntityType{ID: 7}, attrErr: boom}
	et, attr, err := LookupEAVEntityTypeAndAttribute(s, "product", "sku")
	if !errors.Is(err, boom) {
		t.Errorf("attribute lookup error = %v, want boom", err)
	}
	if et == nil || attr != nil {
		t.Errorf("got (%v, %v), want entity and nil attribute", et, attr)
	}
}
