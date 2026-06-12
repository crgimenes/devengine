package db

import (
	"context"
	"errors"
	"testing"

	"github.com/crgimenes/filo"
)

func TestIsValidUsername(t *testing.T) {
	t.Parallel()
	valid := []string{"ana", "Ana_Souza", "user-123", "abc", "a23456789012345678901234567890"}
	for _, u := range valid {
		if err := IsValidUsername(u); err != nil {
			t.Errorf("IsValidUsername(%q) = %v, want nil", u, err)
		}
	}
	invalid := []string{"", "ab", "with space", "acentuação", "semi;colon",
		"a234567890123456789012345678901" /* 31 chars */}
	for _, u := range invalid {
		if err := IsValidUsername(u); err == nil {
			t.Errorf("IsValidUsername(%q) = nil, want error", u)
		}
	}
}

func TestUserErrorIsAnError(t *testing.T) {
	t.Parallel()
	var err error = UserError("authored message")
	if err.Error() != "authored message" {
		t.Fatalf("Error() = %q", err.Error())
	}
	var ue UserError
	if !errors.As(err, &ue) || string(ue) != "authored message" {
		t.Fatalf("errors.As failed: %v", ue)
	}
}

func TestSortElementsHierarchically(t *testing.T) {
	t.Parallel()
	groupID := int64(10)
	elements := []FormElement{
		{ID: 3, MachineName: "child_b", ParentID: &groupID, ZOrder: 2},
		{ID: 1, MachineName: "root_field", ParentID: nil, ZOrder: 0},
		{ID: groupID, MachineName: "group", ParentID: nil, ZOrder: 1},
		{ID: 4, MachineName: "child_a", ParentID: &groupID, ZOrder: 1},
	}

	sorted := SortElementsHierarchically(elements)

	want := []string{"root_field", "group", "child_a", "child_b"}
	if len(sorted) != len(want) {
		t.Fatalf("sorted %d elements, want %d", len(sorted), len(want))
	}
	for i, el := range sorted {
		if el.MachineName != want[i] {
			t.Fatalf("position %d: %q, want %q", i, el.MachineName, want[i])
		}
	}

	// Orphans (parent not in the slice) must not vanish.
	ghostParent := int64(999)
	withOrphan := append(elements, FormElement{ID: 5, MachineName: "orphan", ParentID: &ghostParent})
	sorted = SortElementsHierarchically(withOrphan)
	if len(sorted) != len(withOrphan) {
		t.Fatalf("orphan dropped: %d of %d elements", len(sorted), len(withOrphan))
	}
}

func TestGetDirectChildren(t *testing.T) {
	t.Parallel()
	parent := int64(1)
	other := int64(2)
	items := []MenuItem{
		{ID: 10, MachineName: "a", ParentID: &parent},
		{ID: 11, MachineName: "b", ParentID: &other},
		{ID: 12, MachineName: "c", ParentID: &parent},
		{ID: 13, MachineName: "root", ParentID: nil},
	}
	got := GetDirectChildren(items, parent)
	if len(got) != 2 || got[0].MachineName != "a" || got[1].MachineName != "c" {
		t.Fatalf("GetDirectChildren = %+v", got)
	}
	if len(GetDirectChildren(items, 999)) != 0 {
		t.Fatal("children for unknown parent")
	}
}

func TestExecutePosLoadScript(t *testing.T) {
	t.Parallel()

	// Empty script: values pass through untouched.
	et := &EAVEntityType{ID: 1, Name: "T", PosLoad: ""}
	values := EAVRecordValues{"nome": "ana"}
	out, userErr, err := ExecutePosLoadScript(et, values)
	if err != nil || userErr != "" || out["nome"] != "ana" {
		t.Fatalf("empty pos_load: %v, %q, %v", out, userErr, err)
	}

	// Decoration: pos_load rewrites a field for display.
	et.PosLoad = `(set field:nome (str-upper field:nome))`
	out, userErr, err = ExecutePosLoadScript(et, values)
	if err != nil || userErr != "" {
		t.Fatalf("pos_load: %q, %v", userErr, err)
	}
	if out["nome"] != "ANA" {
		t.Fatalf("pos_load did not decorate: %v", out)
	}

	// Broken script surfaces as an execution error.
	et.PosLoad = `(this-builtin-does-not-exist)`
	_, _, err = ExecutePosLoadScript(et, values)
	if err == nil {
		t.Fatal("broken pos_load must error")
	}
}

func TestExecutePreSaveScriptWithSetup(t *testing.T) {
	t.Parallel()

	et := &EAVEntityType{ID: 1, Name: "T",
		PreSave: `(set field:total (my-builtin field:total))`}
	values := EAVRecordValues{"total": int64(20)}

	setup := func(eng *filo.Engine) {
		eng.MustRegisterBuiltin("my-builtin", func(_ context.Context, args []filo.Value) (filo.Value, error) {
			n, err := args[0].AsNumber()
			if err != nil {
				return filo.Value{}, err
			}
			return filo.VNum(n * 2), nil
		})
	}

	out, userErr, err := ExecutePreSaveScriptWithSetup(et, values, setup)
	if err != nil || userErr != "" {
		t.Fatalf("with setup: %q, %v", userErr, err)
	}
	if out["total"] != int64(40) {
		t.Fatalf("custom builtin not applied: %v", out)
	}
}

func TestRowGuards(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")
	if err := NewErrorRow(sentinel).Scan(); !errors.Is(err, sentinel) {
		t.Fatalf("error row Scan = %v", err)
	}
	if err := NewErrorRow(sentinel).Err(); !errors.Is(err, sentinel) {
		t.Fatalf("error row Err = %v", err)
	}

	var nilRow *Row
	if err := nilRow.Scan(); err == nil {
		t.Fatal("nil row Scan must error")
	}
	if err := nilRow.Err(); err == nil {
		t.Fatal("nil row Err must error")
	}

	empty := &Row{}
	if err := empty.Scan(); err == nil {
		t.Fatal("row without sql.Row must error on Scan")
	}
	if err := empty.Err(); err == nil {
		t.Fatal("row without sql.Row must error on Err")
	}
}
