package handlers

import (
	"net/http"
	"net/url"
	"testing"
)

func TestHandlersRejectCrossOwnerReferences(t *testing.T) {
	mux, store := newHTTPTestEnv(t)
	adminCookie := plantUser(t, "ownership-admin", true)

	entityA, err := store.CreateEAVEntityType("Entity A", "entity_a", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	entityB, err := store.CreateEAVEntityType("Entity B", "entity_b", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	recordB, err := store.CreateEAVRecord(entityB.ID)
	if err != nil {
		t.Fatal(err)
	}

	formA, err := store.CreateForm("form_a", "Form A", "", &entityA.ID)
	if err != nil {
		t.Fatal(err)
	}
	formB, err := store.CreateForm("form_b", "Form B", "", &entityB.ID)
	if err != nil {
		t.Fatal(err)
	}
	elementB, err := store.CreateFormElement(
		formB.ID, nil, "field_b", "field", "Field B", "",
		0, 12, "text", "", nil, true, false,
	)
	if err != nil {
		t.Fatal(err)
	}

	menuA, err := store.CreateMenu("menu_a", "Menu A", "")
	if err != nil {
		t.Fatal(err)
	}
	menuB, err := store.CreateMenu("menu_b", "Menu B", "")
	if err != nil {
		t.Fatal(err)
	}
	itemB, err := store.CreateMenuItem(menuB.ID, nil, "item_b", "Item B", "", "link", "/", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		form url.Values
	}{
		{
			name: "runtime record",
			path: "/form/" + formA.MachineName + "/r/" + recordB.ReferenceID,
			form: url.Values{"rev": {"1"}},
		},
		{
			name: "admin record",
			path: "/tools/database-schema/eav/" + entityA.ReferenceID + "/records/" + recordB.ReferenceID + "/delete",
		},
		{
			name: "form element",
			path: "/tools/forms/" + formA.ReferenceID + "/elements/" + elementB.ReferenceID + "/delete",
		},
		{
			name: "menu item",
			path: "/tools/menu-editor/" + menuA.ReferenceID + "/items/" + itemB.ReferenceID + "/delete",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doPostForm(t, mux, tc.path, tc.form, adminCookie)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
		})
	}

	if _, err := store.GetFormElementByRefID(elementB.ReferenceID); err != nil {
		t.Fatalf("foreign form element was modified: %v", err)
	}
	if _, err := store.GetMenuItemByRefID(itemB.ReferenceID); err != nil {
		t.Fatalf("foreign menu item was modified: %v", err)
	}
	if _, err := store.GetEAVRecordByRefID(recordB.ReferenceID); err != nil {
		t.Fatalf("foreign record was modified: %v", err)
	}
}
