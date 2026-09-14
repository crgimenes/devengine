package db

import (
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
		{AttributeID: 1, VText: strPtr("other")},
		{AttributeID: 2, VText: &v},
	}
	got := FormatEAVValue("TEXT", values, 2, "-")
	if got != "target" {
		t.Errorf("expected 'target', got %q", got)
	}
}

func strPtr(s string) *string { return &s }
