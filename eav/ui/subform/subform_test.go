package subform

import (
	"path/filepath"
	"testing"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/db/sqlite"
	"github.com/crgimenes/devengine/eav/ui"
)

func newStore(t *testing.T) db.Store {
	t.Helper()
	s, err := sqlite.NewWithPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	err = sqlite.RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}
	return s
}

func setStorage(t *testing.T, s db.Store) {
	t.Helper()
	prev := db.Storage
	db.Storage = s
	t.Cleanup(func() { db.Storage = prev })
}

func TestRegistered(t *testing.T) {
	p, ok := ui.Get("subform")
	if !ok {
		t.Fatal("subform plugin not registered")
	}
	if p.ID() != "subform" {
		t.Fatalf("ID = %q", p.ID())
	}
	if p.HasPersistence() {
		t.Fatal("HasPersistence should be false")
	}
}

func TestRecordsForReturnsRelatedRecords(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	setStorage(t, s)

	// Customer entity (parent)
	customer, _ := s.CreateEAVEntityType("Customer", "customer", "", "", "")
	customerName, _ := s.CreateEAVAttribute(customer.ID, "name", "Name", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)

	// Order entity (child) with reference back to customer
	order, _ := s.CreateEAVEntityType("Order", "order", "", "", "")
	orderCustomer, _ := s.CreateEAVAttribute(order.ID, "customer_ref", "Customer", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	orderTitle, _ := s.CreateEAVAttribute(order.ID, "title", "Title", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)

	// Create one customer
	cRec, _ := s.CreateEAVRecord(customer.ID)
	name := "Acme"
	_ = s.UpsertEAVValue(cRec.ID, customerName.ID, nil, nil, nil, &name, nil)

	// Create three orders pointing at the customer
	for _, title := range []string{"order-1", "order-2", "order-3"} {
		rec, _ := s.CreateEAVRecord(order.ID)
		t := title
		ref := cRec.ReferenceID
		_ = s.UpsertEAVValue(rec.ID, orderCustomer.ID, nil, nil, nil, &ref, nil)
		_ = s.UpsertEAVValue(rec.ID, orderTitle.ID, nil, nil, nil, &t, nil)
	}

	// And one order pointing at a different (fake) ref — must not appear
	rec, _ := s.CreateEAVRecord(order.ID)
	other := "different-ref"
	_ = s.UpsertEAVValue(rec.ID, orderCustomer.ID, nil, nil, nil, &other, nil)

	got := recordsFor("order", "customer_ref", cRec.ReferenceID, "title")
	if len(got) != 3 {
		t.Fatalf("got %d records, want 3", len(got))
	}
	seen := map[string]bool{}
	for _, r := range got {
		seen[r.Label] = true
	}
	for _, want := range []string{"order-1", "order-2", "order-3"} {
		if !seen[want] {
			t.Errorf("missing %q", want)
		}
	}
}

func TestRecordsForMissingParentReturnsEmpty(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	setStorage(t, s)

	got := recordsFor("anything", "anything", "", "name")
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestRecordsForUnknownEntityReturnsEmpty(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	setStorage(t, s)

	got := recordsFor("ghost", "attr", "ref", "display")
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
