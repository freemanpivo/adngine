package app_test

import (
	"path/filepath"
	"testing"

	"github.com/freemanpivo/adngine/internal/app"
)

// Prova que a config e os inventarios versionados no repositorio sobem de
// verdade: qualquer arquivo de componente invalido quebra aqui.
func TestNewComAConfigDoRepositorio(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))

	if _, err := app.New(filepath.Join("configs", "config.yaml")); err != nil {
		t.Fatalf("esperava sucesso, recebeu %v", err)
	}
}

func TestNewComConfigInexistente(t *testing.T) {
	if _, err := app.New(filepath.Join(t.TempDir(), "nao_existe.yaml")); err == nil {
		t.Fatal("esperava erro, recebeu nil")
	}
}
