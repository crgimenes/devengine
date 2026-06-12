package db

import (
	"testing"
)

func seedFiveTextRecords(t *testing.T, s *SQLite, labels ...string) (entityID, attrID int64, refs []string) {
	t.Helper()
	et, err := s.CreateEAVEntityType("T", "t", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	attr, err := s.CreateEAVAttribute(et.ID, "title", "Title", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute: %v", err)
	}
	for _, label := range labels {
		rec, err := s.CreateEAVRecord(et.ID)
		if err != nil {
			t.Fatalf("CreateEAVRecord: %v", err)
		}
		title := label
		err = s.UpsertEAVValue(rec.ID, attr.ID, nil, nil, nil, &title, nil)
		if err != nil {
			t.Fatalf("UpsertEAVValue: %v", err)
		}
		refs = append(refs, rec.ReferenceID)
	}
	return et.ID, attr.ID, refs
}

func TestListEAVRecordsCursorPaginates(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	entityID, _, _ := seedFiveTextRecords(t, s, "a", "b", "c", "d", "e")

	first, err := s.ListEAVRecordsCursor(entityID, 0, 2, "")
	if err != nil {
		t.Fatalf("first batch: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first batch len = %d, want 2", len(first))
	}

	second, err := s.ListEAVRecordsCursor(entityID, first[len(first)-1].ID, 2, "")
	if err != nil {
		t.Fatalf("second batch: %v", err)
	}
	if len(second) != 2 {
		t.Fatalf("second batch len = %d", len(second))
	}
	// No overlap
	for _, a := range first {
		for _, b := range second {
			if a.ID == b.ID {
				t.Fatalf("overlap on id %d", a.ID)
			}
		}
	}

	third, _ := s.ListEAVRecordsCursor(entityID, second[len(second)-1].ID, 2, "")
	if len(third) != 1 {
		t.Fatalf("third batch len = %d, want 1", len(third))
	}

	end, _ := s.ListEAVRecordsCursor(entityID, third[len(third)-1].ID, 2, "")
	if len(end) != 0 {
		t.Fatalf("end batch len = %d, want 0 (end of list)", len(end))
	}
}

func TestListEAVRecordsCursorOrdersIDDesc(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	entityID, _, _ := seedFiveTextRecords(t, s, "alpha", "beta", "gamma")

	got, err := s.ListEAVRecordsCursor(entityID, 0, 10, "")
	if err != nil {
		t.Fatalf("ListEAVRecordsCursor: %v", err)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].ID < got[i].ID {
			t.Fatalf("not ordered DESC at index %d: %d < %d", i, got[i-1].ID, got[i].ID)
		}
	}
}

func TestListEAVRecordsCursorFiltersByText(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	entityID, _, _ := seedFiveTextRecords(t, s, "apple", "banana", "applesauce", "cherry", "pineapple")

	hits, err := s.ListEAVRecordsCursor(entityID, 0, 50, "app")
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	// "apple", "applesauce", "pineapple" all contain "app"
	if len(hits) != 3 {
		t.Fatalf("got %d hits, want 3", len(hits))
	}

	noHits, _ := s.ListEAVRecordsCursor(entityID, 0, 50, "zzz")
	if len(noHits) != 0 {
		t.Fatalf("got %d, want 0", len(noHits))
	}
}

func TestListEAVRecordsCursorFilterCaseInsensitive(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	entityID, _, _ := seedFiveTextRecords(t, s, "Hello", "WORLD")

	lower, _ := s.ListEAVRecordsCursor(entityID, 0, 50, "hello")
	upper, _ := s.ListEAVRecordsCursor(entityID, 0, 50, "world")
	if len(lower) != 1 || len(upper) != 1 {
		t.Fatalf("case-insensitive filter broken: lower=%d upper=%d", len(lower), len(upper))
	}
}

func TestGetEAVValuesForRecordIDsBatches(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	entityID, _, _ := seedFiveTextRecords(t, s, "x", "y", "z")
	all, _ := s.ListEAVRecordsCursor(entityID, 0, 10, "")

	ids := make([]int64, len(all))
	for i, r := range all {
		ids[i] = r.ID
	}

	got, err := s.GetEAVValuesForRecordIDs(ids)
	if err != nil {
		t.Fatalf("GetEAVValuesForRecordIDs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got map size %d, want 3", len(got))
	}
	for _, id := range ids {
		if len(got[id]) != 1 {
			t.Errorf("record %d has %d values, want 1", id, len(got[id]))
		}
	}
}

// The text filter must also match through ONE level of reference: a record
// whose TEXT value holds the reference_id of a record whose own text
// matches. Searching "Ana" finds the orders pointing at Ana Souza.
func TestCursorFilterMatchesReferencedRecordText(t *testing.T) {
	s := initTestDB(t)

	clientes, err := s.CreateEAVEntityType("Cliente", "cliente", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	nome, err := s.CreateEAVAttribute(clientes.ID, "nome", "Nome", "", "TEXT",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute: %v", err)
	}
	ana, err := s.CreateEAVRecord(clientes.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord: %v", err)
	}
	anaNome := "Ana Souza"
	err = s.UpsertEAVValue(ana.ID, nome.ID, nil, nil, nil, &anaNome, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue: %v", err)
	}

	pedidos, err := s.CreateEAVEntityType("Pedido", "pedido", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	cli, err := s.CreateEAVAttribute(pedidos.ID, "cliente", "Cliente", "", "TEXT",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute: %v", err)
	}
	pedido, err := s.CreateEAVRecord(pedidos.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord: %v", err)
	}
	err = s.UpsertEAVValue(pedido.ID, cli.ID, nil, nil, nil, &ana.ReferenceID, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue(ref): %v", err)
	}
	// A second order pointing nowhere, to prove filtering still narrows.
	outro, err := s.CreateEAVRecord(pedidos.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord: %v", err)
	}
	texto := "sem cliente"
	err = s.UpsertEAVValue(outro.ID, cli.ID, nil, nil, nil, &texto, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue: %v", err)
	}

	got, err := s.ListEAVRecordsCursor(pedidos.ID, 0, 10, "ana souza")
	if err != nil {
		t.Fatalf("ListEAVRecordsCursor: %v", err)
	}
	if len(got) != 1 || got[0].ID != pedido.ID {
		t.Fatalf("filter through reference = %d records, want the one order", len(got))
	}

	// Direct text still matches as before.
	got, err = s.ListEAVRecordsCursor(pedidos.ID, 0, 10, "sem cliente")
	if err != nil {
		t.Fatalf("ListEAVRecordsCursor: %v", err)
	}
	if len(got) != 1 || got[0].ID != outro.ID {
		t.Fatalf("direct filter = %d records", len(got))
	}

	// And a miss is still a miss.
	got, err = s.ListEAVRecordsCursor(pedidos.ID, 0, 10, "bruno")
	if err != nil {
		t.Fatalf("ListEAVRecordsCursor: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("miss returned %d records", len(got))
	}
}
