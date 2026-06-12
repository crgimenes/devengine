package i18n

import "testing"

// resetLocale restores the default after tests that switch it; dictionaries
// are additive so the built-in pt-BR stays registered.
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

func TestPtBRTranslates(t *testing.T) {
	resetLocale(t)
	SetLocale("pt-BR")

	got := T("Page not found")
	if got != "Página não encontrada" {
		t.Fatalf("T = %q", got)
	}
}

func TestMissingEntryFallsBackToEnglish(t *testing.T) {
	resetLocale(t)
	SetLocale("pt-BR")

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
	SetLocale("pt-BR")

	got := T("Could not update (ref %s)", "ab12")
	if got != "Erro ao atualizar (ref ab12)" {
		t.Fatalf("T = %q", got)
	}
}

func TestRegisterMergesAndOverrides(t *testing.T) {
	resetLocale(t)
	Register("pt-BR", map[string]string{"App only string": "String só do app"})
	SetLocale("pt-BR")

	got := T("App only string")
	if got != "String só do app" {
		t.Fatalf("T = %q", got)
	}
	// Engine entries keep working after the merge.
	if T("Invalid credentials.") != "Credenciais inválidas." {
		t.Fatal("engine entry lost after Register")
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
		{"pt-BR", "pt-BR"},
		{"pt-br", "pt-BR"},
		{"pt", "pt-BR"},
		{"pt-PT", "pt-BR"}, // language prefix match
		{"en-US,en;q=0.9", "en-US"},
		{"en-GB,en;q=0.9", "en-US"},
		{"fr-FR,pt-BR;q=0.8", "pt-BR"}, // first known wins, in header order
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
		t.Fatalf("Locales = %v, want at least en-US and pt-BR", locs)
	}
	if !Known("en-US") || !Known("pt-BR") {
		t.Fatal("built-in locales not known")
	}
	if Known("ja-JP") {
		t.Fatal("unknown locale reported as known")
	}
}
