package conversation_test

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/freemanpivo/adngine/internal/conversation"
	"github.com/freemanpivo/adngine/internal/testsupport"
)

func ids(cs []conversation.Conversation) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

func assertIDs(t *testing.T, got []conversation.Conversation, want ...string) {
	t.Helper()

	gotIDs := ids(got)
	if len(gotIDs) != len(want) {
		t.Fatalf("esperava %v, recebeu %v", want, gotIDs)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("esperava %v, recebeu %v", want, gotIDs)
		}
	}
}

func TestRepositoryByComponent(t *testing.T) {
	repo := testsupport.LoadRepository(t, "inventory_valid.yaml")

	tests := []struct {
		name      string
		component string
		want      []string
	}{
		{
			name:      "banner mantem a ordem do arquivo",
			component: "banner",
			want:      []string{"conv-veiculo-alta", "conv-veiculo-baixa", "conv-eletro-alta", "conv-generica"},
		},
		{
			name:      "card ignora conversa exclusiva de outro componente",
			component: "card",
			want:      []string{"conv-veiculo-alta", "conv-veiculo-baixa", "conv-generica"},
		},
		{
			name:      "footer",
			component: "footer",
			want:      []string{"conv-generica", "conv-empate-a", "conv-empate-b"},
		},
		{
			name:      "componente desconhecido nao devolve nada",
			component: "sidebar",
			want:      nil,
		},
		{
			name:      "componente vazio nao devolve nada",
			component: "",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertIDs(t, repo.ByComponent(tt.component), tt.want...)
		})
	}
}

func TestRepositoryByComponentPreservaCampos(t *testing.T) {
	repo := testsupport.LoadRepository(t, "inventory_valid.yaml")

	got := repo.ByComponent("banner")
	if len(got) == 0 {
		t.Fatal("esperava candidatos para banner")
	}

	first := got[0]
	if first.ID != "conv-veiculo-alta" {
		t.Fatalf("esperava conv-veiculo-alta como primeira, recebeu %s", first.ID)
	}
	if first.Type != conversation.TypeAction {
		t.Errorf("type: esperava %q, recebeu %q", conversation.TypeAction, first.Type)
	}
	if first.Product != "veiculo" {
		t.Errorf("product: esperava %q, recebeu %q", "veiculo", first.Product)
	}
	if first.Priority != 10 {
		t.Errorf("priority: esperava 10, recebeu %d", first.Priority)
	}
	if first.Text == "" || first.Link == "" {
		t.Error("text e link devem ser carregados")
	}
}

func TestRepositoryLoadInventarioVazio(t *testing.T) {
	repo := testsupport.LoadRepository(t, "inventory_empty.yaml")

	if got := repo.ByComponent("banner"); len(got) != 0 {
		t.Fatalf("esperava nenhum candidato, recebeu %v", ids(got))
	}
}

func TestRepositoryLoadErros(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "arquivo valido", path: testsupport.FixturePath(t, "inventory_valid.yaml"), wantErr: false},
		{name: "arquivo inexistente", path: filepath.Join(t.TempDir(), "nao_existe.yaml"), wantErr: true},
		{name: "yaml malformado", path: testsupport.FixturePath(t, "inventory_malformed.yaml"), wantErr: true},
		{name: "tipo de campo invalido", path: testsupport.FixturePath(t, "inventory_wrong_types.yaml"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := conversation.NewRepository().Load(tt.path)
			if tt.wantErr && err == nil {
				t.Fatal("esperava erro, recebeu nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("esperava sucesso, recebeu %v", err)
			}
		})
	}
}

func TestRepositoryLoadSubstituiInventario(t *testing.T) {
	repo := testsupport.LoadRepository(t, "inventory_valid.yaml")
	if got := repo.ByComponent("banner"); len(got) == 0 {
		t.Fatal("esperava candidatos antes da recarga")
	}

	if err := repo.Load(testsupport.FixturePath(t, "inventory_empty.yaml")); err != nil {
		t.Fatalf("recarregando: %v", err)
	}
	if got := repo.ByComponent("banner"); len(got) != 0 {
		t.Fatalf("recarga deveria substituir o inventario, recebeu %v", ids(got))
	}
}

func TestRepositoryLeituraConcorrente(t *testing.T) {
	repo := testsupport.LoadRepository(t, "inventory_valid.yaml")

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				if got := repo.ByComponent("banner"); len(got) != 4 {
					t.Errorf("esperava 4 candidatos, recebeu %d", len(got))
					return
				}
			}
		}()
	}
	wg.Wait()
}
