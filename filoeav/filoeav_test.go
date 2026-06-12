package filoeav

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/db/sqlite"
	"github.com/crgimenes/filo"
)

func newTestStore(t *testing.T) db.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := sqlite.NewWithPath(path)
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	err = sqlite.RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}
	return s
}

func seedPerson(t *testing.T, s db.Store) (entityID int64, nameAttrID int64) {
	t.Helper()
	et, err := s.CreateEAVEntityType("Person", "person", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	attr, err := s.CreateEAVAttribute(et.ID, "name", "Name", "", "TEXT", false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute: %v", err)
	}
	return et.ID, attr.ID
}

func insertRecordWithName(t *testing.T, s db.Store, entityID, attrID int64, name string) string {
	t.Helper()
	rec, err := s.CreateEAVRecord(entityID)
	if err != nil {
		t.Fatalf("CreateEAVRecord: %v", err)
	}
	err = s.UpsertEAVValue(rec.ID, attrID, nil, nil, nil, &name, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue: %v", err)
	}
	return rec.ReferenceID
}

func TestExistsAndCount(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	eID, attrID := seedPerson(t, s)
	refAlice := insertRecordWithName(t, s, eID, attrID, "Alice")
	insertRecordWithName(t, s, eID, attrID, "Bob")

	c := NewContext(s)

	exists, err := c.exists(context.Background(), []filo.Value{filo.VString("person"), filo.VString(refAlice)})
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !exists.Bool {
		t.Error("exists = false for known record")
	}

	exists, err = c.exists(context.Background(), []filo.Value{filo.VString("person"), filo.VString("nope")})
	if err != nil {
		t.Fatalf("exists missing: %v", err)
	}
	if exists.Bool {
		t.Error("exists = true for missing record")
	}

	count, err := c.count(context.Background(), []filo.Value{filo.VString("person")})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if int64(count.Num) != 2 {
		t.Errorf("count = %v, want 2", count.Num)
	}
}

func TestCountWhereText(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	eID, attrID := seedPerson(t, s)
	insertRecordWithName(t, s, eID, attrID, "Alice")
	insertRecordWithName(t, s, eID, attrID, "Alice")
	insertRecordWithName(t, s, eID, attrID, "Bob")

	c := NewContext(s)

	n, err := c.countWhere(context.Background(), []filo.Value{
		filo.VString("person"), filo.VString("name"), filo.VString("Alice"),
	})
	if err != nil {
		t.Fatalf("countWhere: %v", err)
	}
	if int64(n.Num) != 2 {
		t.Errorf("countWhere(Alice) = %v, want 2", n.Num)
	}

	n, err = c.countWhere(context.Background(), []filo.Value{
		filo.VString("person"), filo.VString("name"), filo.VString("Eve"),
	})
	if err != nil {
		t.Fatalf("countWhere(Eve): %v", err)
	}
	if int64(n.Num) != 0 {
		t.Errorf("countWhere(Eve) = %v, want 0", n.Num)
	}
}

func TestGetValue(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	eID, attrID := seedPerson(t, s)
	refAlice := insertRecordWithName(t, s, eID, attrID, "Alice")

	c := NewContext(s)

	v, err := c.getValue(context.Background(), []filo.Value{
		filo.VString("person"), filo.VString(refAlice), filo.VString("name"),
	})
	if err != nil {
		t.Fatalf("getValue: %v", err)
	}
	if v.Str != "Alice" {
		t.Errorf("getValue = %q, want Alice", v.Str)
	}

	v, err = c.getValue(context.Background(), []filo.Value{
		filo.VString("person"), filo.VString("missing"), filo.VString("name"),
	})
	if err != nil {
		t.Fatalf("getValue missing: %v", err)
	}
	if v.Str != "" {
		t.Errorf("getValue missing = %q, want empty", v.Str)
	}
}

func TestUnknownEntity(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	c := NewContext(s)
	exists, err := c.exists(context.Background(), []filo.Value{filo.VString("ghost"), filo.VString("x")})
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if exists.Bool {
		t.Error("exists on unknown entity = true")
	}
}

func TestRejectsBadArity(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	c := NewContext(s)
	_, err := c.count(context.Background(), nil)
	if err == nil {
		t.Fatal("expected arity error")
	}
}
