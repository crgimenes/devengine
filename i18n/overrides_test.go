package i18n

import (
	"slices"
	"sync"
	"testing"
)

func TestOverrideLayerWinsAndRestores(t *testing.T) {
	const key = "Save"
	builtin := TL("pt-BR", key)

	SetOverride("pt-BR", key, "Gravar")
	t.Cleanup(func() { DeleteOverride("pt-BR", key) })

	if got := TL("pt-BR", key); got != "Gravar" {
		t.Fatalf("override ignored: %q", got)
	}
	if got, ok := Override("pt-BR", key); !ok || got != "Gravar" {
		t.Fatalf("Override = %q, %v", got, ok)
	}

	// Deleting restores the built-in dictionary translation.
	DeleteOverride("pt-BR", key)
	if got := TL("pt-BR", key); got != builtin {
		t.Fatalf("delete did not restore built-in: %q != %q", got, builtin)
	}
	if _, ok := Override("pt-BR", key); ok {
		t.Fatal("deleted override still present")
	}
}

func TestOverrideForUnknownLocale(t *testing.T) {
	SetOverride("xx-XX", "Hello", "Custom")
	t.Cleanup(func() { DeleteOverride("xx-XX", "Hello") })

	if got := TL("xx-XX", "Hello"); got != "Custom" {
		t.Fatalf("override on dictionary-less locale = %q", got)
	}
}

func TestKeysListsDictionary(t *testing.T) {
	keys := Keys()
	if len(keys) == 0 {
		t.Fatal("dictionary keys reported empty")
	}
	if !slices.IsSorted(keys) {
		t.Fatal("Keys must come sorted")
	}
}

func TestContentLayer(t *testing.T) {
	SetContent("en-US", "ref-1", "label", "Customers")
	t.Cleanup(func() { DeleteContent("en-US", "ref-1", "label") })

	if got, ok := Content("en-US", "ref-1", "label"); !ok || got != "Customers" {
		t.Fatalf("Content = %q, %v", got, ok)
	}
	if got := ContentOr("en-US", "ref-1", "label", "original"); got != "Customers" {
		t.Fatalf("ContentOr = %q", got)
	}
	// Missing translation falls back to the original text.
	if got := ContentOr("en-US", "ref-1", "help_text", "original"); got != "original" {
		t.Fatalf("ContentOr fallback = %q", got)
	}

	DeleteContent("en-US", "ref-1", "label")
	if _, ok := Content("en-US", "ref-1", "label"); ok {
		t.Fatal("deleted content still present")
	}
}

// The translation maps are read on every request and written by the tools
// screens; hammer them in parallel to prove the locking holds.
func TestConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			for range 200 {
				SetOverride("pt-BR", "hammer", "x")
				DeleteOverride("pt-BR", "hammer")
				SetContent("pt-BR", "ref-h", "label", "y")
				DeleteContent("pt-BR", "ref-h", "label")
			}
		}(i)
		go func() {
			defer wg.Done()
			for range 200 {
				_ = TL("pt-BR", "Save")
				_ = ContentOr("pt-BR", "ref-h", "label", "z")
				_ = Keys()
			}
		}()
	}
	wg.Wait()
}
