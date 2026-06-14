package assets

import (
	"io/fs"
	"strings"
	"testing"
)

func TestEmbeddedAssetsContainNoLegacyPortugueseUI(t *testing.T) {
	markers := []string{
		"selecione um",
		"selecionar ",
		"enviando...",
		"carregando...",
		"nenhum ",
		"nenhuma ",
		"erro ao ",
		"upload falhou",
		"upload concluído",
		"role para ",
		"tem certeza ",
		">ativo<",
		">rascunho<",
		"criado:",
		"'sim'",
		"'não'",
		"muitas tags",
		"tag muito longa",
		"caracteres inválidos",
		"tags inválidas",
		"adicionar categoria",
		"deve ser um número",
		"tente novamente",
		"erro de conexão",
		"erro na requisicao",
		"string vazia",
	}

	err := fs.WalkDir(assets, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}

		content, err := fs.ReadFile(assets, path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(content))
		for _, marker := range markers {
			if strings.Contains(lower, marker) {
				t.Errorf("%s contains legacy Portuguese UI text %q", path, marker)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
