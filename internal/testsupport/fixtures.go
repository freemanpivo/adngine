// Package testsupport carrega os inventarios de teste compartilhados pelos
// demais pacotes.
package testsupport

import (
	"log/slog"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/freemanpivo/adngine/internal/conversation"
)

// Components sao os componentes cobertos pelas fixtures em fixtures/valid.
var Components = []string{"banner", "card", "footer"}

// FixturePath resolve o caminho a partir da localizacao deste arquivo, e nao do
// diretorio do teste que chama, para que qualquer pacote use as mesmas fixtures.
func FixturePath(t *testing.T, rel string) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("testsupport: nao foi possivel resolver o caminho do pacote")
	}
	return filepath.Join(filepath.Dir(thisFile), "fixtures", rel)
}

func Logger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// LoadInventory monta um repositorio com os tres componentes de fixtures/valid.
func LoadInventory(t *testing.T) *conversation.Repository {
	t.Helper()

	repo := conversation.NewRepository(Logger())
	for _, component := range Components {
		LoadComponentInto(t, repo, component, filepath.Join("valid", component+".yaml"))
	}
	return repo
}

func LoadComponent(t *testing.T, component, rel string) *conversation.Repository {
	t.Helper()

	repo := conversation.NewRepository(Logger())
	LoadComponentInto(t, repo, component, rel)
	return repo
}

func LoadComponentInto(t *testing.T, repo *conversation.Repository, component, rel string) {
	t.Helper()

	if err := repo.LoadComponent(component, FixturePath(t, rel)); err != nil {
		t.Fatalf("carregando fixture %s como %s: %v", rel, component, err)
	}
}
