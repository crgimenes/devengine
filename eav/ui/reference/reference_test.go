package reference

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

func seedPersonEntity(t *testing.T, s db.Store) (entityID, nameAttrID int64) {
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

func seedPersonRecord(t *testing.T, s db.Store, entityID, attrID int64, name string) string {
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

func seedPerson(t *testing.T, s db.Store, name string) string {
	t.Helper()
	eID, aID := seedPersonEntity(t, s)
	return seedPersonRecord(t, s, eID, aID, name)
}

func TestRegistered(t *testing.T) {
	p, ok := ui.Get("reference")
	if !ok {
		t.Fatal("reference plugin not registered")
	}
	if p.ID() != "reference" {
		t.Fatalf("ID = %q", p.ID())
	}
}

func TestParseTrims(t *testing.T) {
	p, _ := ui.Get("reference")
	v, err := p.Parse(" abc-123 ", nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if v != "abc-123" {
		t.Fatalf("Parse = %q", v)
	}
}

func TestValidateRejectsEmptyByDefault(t *testing.T) {
	p, _ := ui.Get("reference")
	opts := p.ParseOptions(`{"entity": "person"}`)
	err := p.Validate("", opts)
	if err == nil {
		t.Fatal("Validate(\"\") expected error when allow_empty=false")
	}
}

func TestValidateAllowsEmptyWhenConfigured(t *testing.T) {
	p, _ := ui.Get("reference")
	opts := p.ParseOptions(`{"entity": "person", "allow_empty": true}`)
	err := p.Validate("", opts)
	if err != nil {
		t.Fatalf("Validate(\"\") with allow_empty: %v", err)
	}
}

func TestValidateRejectsMissingEntity(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	setStorage(t, s)

	p, _ := ui.Get("reference")
	opts := p.ParseOptions(`{"entity": "ghost"}`)
	err := p.Validate("any", opts)
	if err == nil {
		t.Fatal("Validate with non-existent entity expected error")
	}
}

func TestValidateAcceptsExistingRecord(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	setStorage(t, s)

	ref := seedPerson(t, s, "Alice")

	p, _ := ui.Get("reference")
	opts := p.ParseOptions(`{"entity": "person"}`)
	err := p.Validate(ref, opts)
	if err != nil {
		t.Fatalf("Validate(%q): %v", ref, err)
	}
}

func TestValidateRejectsRecordOfDifferentEntity(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	setStorage(t, s)

	personRef := seedPerson(t, s, "Bob")
	other, _ := s.CreateEAVEntityType("Doc", "doc", "", "", "")
	_, _ = s.CreateEAVRecord(other.ID)

	p, _ := ui.Get("reference")
	opts := p.ParseOptions(`{"entity": "doc"}`)
	err := p.Validate(personRef, opts)
	if err == nil {
		t.Fatal("Validate accepted ref from a different entity")
	}
}

func TestChoicesForReturnsLabelsFromDisplayAttr(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	setStorage(t, s)

	eID, aID := seedPersonEntity(t, s)
	seedPersonRecord(t, s, eID, aID, "Carol")
	seedPersonRecord(t, s, eID, aID, "Dave")

	got := choicesFor("person", "name")
	if len(got) != 2 {
		t.Fatalf("choices = %d, want 2", len(got))
	}
	seen := map[string]bool{}
	for _, c := range got {
		seen[c.Label] = true
		if c.Value == "" {
			t.Errorf("choice missing reference_id: %+v", c)
		}
	}
	if !seen["Carol"] || !seen["Dave"] {
		t.Fatalf("labels missing names: %+v", got)
	}
}

func TestChoicesForUnknownEntityReturnsEmpty(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	setStorage(t, s)

	got := choicesFor("ghost", "name")
	if len(got) != 0 {
		t.Fatalf("choices = %d, want 0", len(got))
	}
}
