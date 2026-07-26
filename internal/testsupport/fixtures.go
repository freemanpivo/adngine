// Package testsupport carrega os inventarios de teste compartilhados pelos
// demais pacotes.
package testsupport

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/freemanpivo/adngine/internal/conversation"
)

// FixturePath resolve o caminho a partir da localizacao deste arquivo, e nao do
// diretorio do teste que chama, para que qualquer pacote use as mesmas fixtures.
func FixturePath(t *testing.T, name string) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("testsupport: nao foi possivel resolver o caminho do pacote")
	}
	return filepath.Join(filepath.Dir(thisFile), "fixtures", name)
}

func LoadRepository(t *testing.T, name string) *conversation.Repository {
	t.Helper()

	repo := conversation.NewRepository()
	if err := repo.Load(FixturePath(t, name)); err != nil {
		t.Fatalf("carregando fixture %s: %v", name, err)
	}
	return repo
}
