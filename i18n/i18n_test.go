package i18n

import "testing"

// A throwaway second locale exercises the translation mechanism now that the
// built-in pt-BR dictionary is disabled (see pt_br.go translationsEnabled).
// Registered only in the test binary, it never leaks into the engine.
func init() {
	Register("tt-TT", map[string]string{"Page not found": "Pagina nao encontrada (tt)"})
}

// resetLocale restores the default after tests that switch it; dictionaries
// are additive so a registered locale stays registered.
func resetLocale(t *testing.T) {
	t.Cleanup(func() { SetLocale(DefaultLocale) })
}

func TestDefaultLocalePassesThrough(t *testing.T) {
	resetLocale(t)
	SetLocale(DefaultLocale)

	got := T("Page not found")
	if got != "Page not found" {
		t.Fatalf("T = %q, want passthrough", got)
	}
}

func TestRegisteredLocaleTranslates(t *testing.T) {
	resetLocale(t)
	SetLocale("tt-TT")

	got := T("Page not found")
	if got != "Pagina nao encontrada (tt)" {
		t.Fatalf("T = %q", got)
	}
}

func TestMissingEntryFallsBackToEnglish(t *testing.T) {
	resetLocale(t)
	SetLocale("tt-TT")

	got := T("Untranslated engine string")
	if got != "Untranslated engine string" {
		t.Fatalf("T = %q, want English fallback", got)
	}
}

func TestUnknownLocaleBehavesAsEnglish(t *testing.T) {
	resetLocale(t)
	SetLocale("ja-JP")

	got := T("Page not found")
	if got != "Page not found" {
		t.Fatalf("T = %q", got)
	}
}

func TestFormatArgsApplyAfterTranslation(t *testing.T) {
	resetLocale(t)
	// Use a throwaway locale so the test exercises the format-after-translate
	// mechanism regardless of which built-in dictionaries are enabled.
	Register("xx-XX", map[string]string{"Could not update (ref %s)": "Falhou (ref %s)"})
	SetLocale("xx-XX")

	got := T("Could not update (ref %s)", "ab12")
	if got != "Falhou (ref ab12)" {
		t.Fatalf("T = %q", got)
	}
}

func TestRegisterMergesAndOverrides(t *testing.T) {
	resetLocale(t)
	Register("yy-YY", map[string]string{"First": "Primeiro"})
	Register("yy-YY", map[string]string{"Second": "Segundo"}) // additive merge
	SetLocale("yy-YY")

	// Both entries resolve: the second Register merges, it does not replace.
	if T("First") != "Primeiro" {
		t.Fatal("first entry lost after a second Register (merge not additive)")
	}
	if T("Second") != "Segundo" {
		t.Fatalf("T(Second) = %q", T("Second"))
	}
}

func TestEmptyLocaleResetsToDefault(t *testing.T) {
	resetLocale(t)
	SetLocale("pt-BR")
	SetLocale("")
	if Locale() != DefaultLocale {
		t.Fatalf("Locale = %q", Locale())
	}
}

func TestMatchHeader(t *testing.T) {
	resetLocale(t)

	cases := []struct{ header, want string }{
		{"tt-TT", "tt-TT"},
		{"tt-tt", "tt-TT"},
		{"tt", "tt-TT"},
		{"tt-XX", "tt-TT"}, // language prefix match
		{"en-US,en;q=0.9", "en-US"},
		{"en-GB,en;q=0.9", "en-US"},
		{"fr-FR,tt-TT;q=0.8", "tt-TT"}, // first known wins, in header order
		{"ja-JP", ""},
		{"*", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := MatchHeader(c.header)
		if got != c.want {
			t.Fatalf("MatchHeader(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

func TestLocalesAndKnown(t *testing.T) {
	resetLocale(t)

	locs := Locales()
	if len(locs) < 2 {
		t.Fatalf("Locales = %v, want at least en-US and the test locale", locs)
	}
	if !Known("en-US") || !Known("tt-TT") {
		t.Fatal("expected locales not known")
	}
	if Known("ja-JP") {
		t.Fatal("unknown locale reported as known")
	}
}
