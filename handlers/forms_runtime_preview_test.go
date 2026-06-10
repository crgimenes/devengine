package handlers

import (
	"context"
	"testing"

	"github.com/crgimenes/devengine/db"
)

func TestApplyComputedExprsViaPreviewPath(t *testing.T) {
	// This exercises the same code path the preview handler hits — useful
	// even without spinning up a real HTTP request.
	s := newValidateTestStore(t)
	defer s.Close()
	setStorage(t, s)

	et, _ := s.CreateEAVEntityType("Order", "order", "", "", "")
	code, err := s.CreateEAVAttribute(et.ID, "code", "Code", "", "TEXT",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(code): %v", err)
	}
	label, err := s.CreateEAVAttribute(et.ID, "label", "Label", "", "TEXT",
		false, false, false, nil, true, `(str-concat "order-" field:code)`, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(label): %v", err)
	}

	values := db.EAVRecordValues{"code": "42", "label": ""}
	got, err := applyComputedExprs(context.Background(), nil,
		[]db.EAVAttribute{*code, *label}, values)
	if err != nil {
		t.Fatalf("applyComputedExprs: %v", err)
	}
	if got["label"] != "order-42" {
		t.Fatalf("label = %q, want order-42", got["label"])
	}
}
