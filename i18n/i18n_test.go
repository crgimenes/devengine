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
