package ui_test

import (
	"slices"
	"testing"

	"github.com/crgimenes/devengine/eav/ui"
)

type stubPlugin struct{ id string }

func (p stubPlugin) ID() string                   { return p.id }
func (stubPlugin) PrimitiveKinds() []string       { return []string{"TEXT"} }
func (stubPlugin) HasPersistence() bool           { return true }
func (stubPlugin) SupportsReadOnly() bool         { return true }
func (stubPlugin) ParseOptions(string) any        { return nil }
func (stubPlugin) Parse(string, any) (any, error) { return nil, nil }
func (stubPlugin) Validate(any, any) error        { return nil }

func TestRegisterAndGet(t *testing.T) {
	ui.Reset()
	t.Cleanup(ui.Reset)

	ui.Register("foo", func() ui.FieldUI { return stubPlugin{id: "foo"} })

	p, ok := ui.Get("foo")
	if !ok {
		t.Fatal("Get returned ok=false for registered plugin")
	}
	if p.ID() != "foo" {
		t.Fatalf("plugin id = %q, want %q", p.ID(), "foo")
	}

	_, ok = ui.Get("missing")
	if ok {
		t.Fatal("Get returned ok=true for unregistered plugin")
	}
}

func TestIDsSorted(t *testing.T) {
	ui.Reset()
	t.Cleanup(ui.Reset)

	for _, id := range []string{"zeta", "alpha", "mu"} {
		ui.Register(id, func() ui.FieldUI { return stubPlugin{id: id} })
	}

	got := ui.IDs()
	want := []string{"alpha", "mu", "zeta"}
	if !slices.Equal(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestRegisterRejectsEmptyID(t *testing.T) {
	ui.Reset()
	t.Cleanup(ui.Reset)

	defer func() {
		if recover() == nil {
			t.Fatal("Register(\"\", ...) did not panic")
		}
	}()
	ui.Register("", func() ui.FieldUI { return stubPlugin{} })
}

func TestRegisterRejectsNilFactory(t *testing.T) {
	ui.Reset()
	t.Cleanup(ui.Reset)

	defer func() {
		if recover() == nil {
			t.Fatal("Register(id, nil) did not panic")
		}
	}()
	ui.Register("foo", nil)
}

func TestFactoryReturnsFreshInstance(t *testing.T) {
	ui.Reset()
	t.Cleanup(ui.Reset)

	count := 0
	ui.Register("foo", func() ui.FieldUI {
		count++
		return stubPlugin{id: "foo"}
	})

	_, _ = ui.Get("foo")
	_, _ = ui.Get("foo")
	if count != 2 {
		t.Fatalf("factory called %d times, want 2", count)
	}
}
