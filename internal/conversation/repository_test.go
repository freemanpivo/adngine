package conversation_test

import (
	"path/filepath"
	"strings"
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

func TestRepositoryCandidates(t *testing.T) {
	repo := testsupport.LoadInventory(t)

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
			name:      "card so enxerga o proprio inventario",
			component: "card",
			want:      []string{"conv-veiculo-alta", "conv-veiculo-baixa", "conv-generica"},
		},
		{
			name:      "footer",
			component: "footer",
			want:      []string{"conv-generica", "conv-empate-a", "conv-empate-b"},
		},
		{
			name:      "componente nao carregado nao devolve nada",
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
			assertIDs(t, repo.Candidates(tt.component), tt.want...)
		})
	}
}

// O mesmo id em componentes diferentes e legitimo: os inventarios sao
// independentes e podem divergir.
func TestRepositoryComponentesSaoIndependentes(t *testing.T) {
	repo := testsupport.LoadInventory(t)

	banner := repo.Candidates("banner")
	card := repo.Candidates("card")

	if len(banner) == len(card) {
		t.Fatalf("fixtures deveriam ter inventarios distintos: banner=%v card=%v", ids(banner), ids(card))
	}
	if got := repo.Components(); len(got) != 3 || got[0] != "banner" || got[1] != "card" || got[2] != "footer" {
		t.Fatalf("Components deve vir ordenado, recebeu %v", got)
	}
}

func TestRepositoryCandidatesPreservaCampos(t *testing.T) {
	repo := testsupport.LoadInventory(t)

	got := repo.Candidates("banner")
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
	if first.Source() != conversation.SourceStatic {
		t.Errorf("conversa sem eligibility deve ser static, recebeu %q", first.Source())
	}
}

func TestRepositoryCandidatesDevolveCopia(t *testing.T) {
	repo := testsupport.LoadInventory(t)

	first := repo.Candidates("banner")
	first[0].Priority = -1
	first[0].Text = "corrompido"

	second := repo.Candidates("banner")
	if second[0].Priority != 10 || second[0].Text == "corrompido" {
		t.Fatalf("inventario corrompido pelo consumidor: %+v", second[0])
	}
}

func TestRepositoryFallback(t *testing.T) {
	repo := testsupport.LoadInventory(t)

	tests := []struct {
		name      string
		component string
		product   string
		want      string
	}{
		{
			name:      "produto com fallback proprio",
			component: "banner",
			product:   "veiculo",
			want:      "fb-banner-veiculo",
		},
		{
			name:      "produto sem fallback proprio cai no default",
			component: "banner",
			product:   "eletrodomestico",
			want:      "fb-banner-default",
		},
		{
			name:      "cliente sem produto recebe o default",
			component: "banner",
			product:   "",
			want:      "fb-banner-default",
		},
		{
			name:      "componente que so tem default",
			component: "card",
			product:   "veiculo",
			want:      "fb-card-default",
		},
		{
			name:      "componente nao carregado nao tem fallback",
			component: "sidebar",
			product:   "veiculo",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := repo.Fallback(tt.component, tt.product)

			if tt.want == "" {
				if ok {
					t.Fatalf("esperava nenhum fallback, recebeu %s", got.ID)
				}
				return
			}
			if !ok {
				t.Fatalf("esperava %s, nao houve fallback", tt.want)
			}
			if got.ID != tt.want {
				t.Fatalf("esperava %s, recebeu %s", tt.want, got.ID)
			}
		})
	}
}

func TestRepositoryFallbackDevolveCopia(t *testing.T) {
	repo := testsupport.LoadInventory(t)

	first, ok := repo.Fallback("banner", "veiculo")
	if !ok {
		t.Fatal("esperava um fallback")
	}
	first.Text = "corrompido"

	second, _ := repo.Fallback("banner", "veiculo")
	if second.Text == "corrompido" {
		t.Fatal("fallback em memoria corrompido pelo consumidor")
	}
}

func TestRepositoryLoadComponentInventarioVazio(t *testing.T) {
	repo := testsupport.LoadComponent(t, "banner", filepath.Join("empty", "banner.yaml"))

	if got := repo.Candidates("banner"); len(got) != 0 {
		t.Fatalf("esperava nenhum candidato, recebeu %v", ids(got))
	}
	if _, ok := repo.Fallback("banner", "veiculo"); !ok {
		t.Fatal("inventario vazio ainda deve ter fallback")
	}
}

func TestRepositoryLoadComponentErros(t *testing.T) {
	tests := []struct {
		name      string
		component string
		rel       string
		wantIn    string
	}{
		{name: "yaml malformado", component: "banner", rel: "invalid/malformed.yaml"},
		{name: "tipo de campo invalido", component: "banner", rel: "invalid/wrong_types.yaml"},
		{
			name:      "sem fallback default",
			component: "banner",
			rel:       "invalid/no_default_fallback.yaml",
			wantIn:    "fallback default",
		},
		{
			name:      "dois fallbacks para o mesmo produto",
			component: "banner",
			rel:       "invalid/duplicate_fallback_product.yaml",
			wantIn:    "ja existe outro fallback para o produto",
		},
		{
			name:      "id repetido entre conversa e fallback",
			component: "banner",
			rel:       "invalid/duplicate_ids.yaml",
			wantIn:    "id duplicado",
		},
		{
			name:      "componente declarado diferente do carregado",
			component: "banner",
			rel:       "invalid/component_mismatch.yaml",
			wantIn:    "componente declarado",
		},
		{
			name:      "fallback com eligibility",
			component: "banner",
			rel:       "invalid/fallback_with_eligibility.yaml",
			wantIn:    "nao pode declarar eligibility",
		},
		{
			name:      "source desconhecida",
			component: "banner",
			rel:       "invalid/unknown_source.yaml",
			wantIn:    "source desconhecida",
		},
		{
			name:      "regra de elegibilidade invalida",
			component: "banner",
			rel:       "invalid/invalid_rule.yaml",
			wantIn:    `conversa "conv-regra-quebrada": regra invalida`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := conversation.NewRepository(testsupport.Logger())

			err := repo.LoadComponent(tt.component, testsupport.FixturePath(t, tt.rel))
			if err == nil {
				t.Fatal("esperava erro, recebeu nil")
			}
			if tt.wantIn != "" && !strings.Contains(err.Error(), tt.wantIn) {
				t.Fatalf("erro deveria conter %q, recebeu %q", tt.wantIn, err)
			}
			if !strings.Contains(err.Error(), tt.component) {
				t.Errorf("erro deveria nomear o componente %q: %q", tt.component, err)
			}
		})
	}
}

// Uma carga quebrada deve mostrar tudo o que ha de errado de uma vez.
func TestRepositoryLoadComponentAcumulaViolacoes(t *testing.T) {
	repo := conversation.NewRepository(testsupport.Logger())

	err := repo.LoadComponent("banner", testsupport.FixturePath(t, "invalid/missing_fields.yaml"))
	if err == nil {
		t.Fatal("esperava erro, recebeu nil")
	}

	for _, want := range []string{"text e obrigatorio", "id e obrigatorio", "type invalido"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("erro deveria conter %q, recebeu %q", want, err)
		}
	}
}

// Uma regra quebrada tambem reporta todas as violacoes de uma vez, nomeando a
// conversa.
func TestRepositoryLoadComponentAcumulaViolacoesDeRegra(t *testing.T) {
	repo := conversation.NewRepository(testsupport.Logger())

	err := repo.LoadComponent("banner", testsupport.FixturePath(t, "invalid/invalid_rule.yaml"))
	if err == nil {
		t.Fatal("esperava erro, recebeu nil")
	}

	wants := []string{
		"conv-regra-quebrada",
		`operador desconhecido "aproximadamente"`,
		"field e obrigatorio",
		`operador "in" exige uma lista em value`,
		"regex invalida",
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("erro deveria conter %q, recebeu %q", want, err)
		}
	}
}

func TestRepositoryLoadComponentArquivoInexistente(t *testing.T) {
	repo := conversation.NewRepository(testsupport.Logger())

	if err := repo.LoadComponent("banner", filepath.Join(t.TempDir(), "nao_existe.yaml")); err == nil {
		t.Fatal("esperava erro, recebeu nil")
	}
}

// O formato antigo tinha `components` dentro da conversa. O campo agora e
// ignorado; a carga nao pode falhar por causa dele.
func TestRepositoryLoadComponentIgnoraCampoLegado(t *testing.T) {
	repo := testsupport.LoadComponent(t, "banner", "invalid/legacy_components_field.yaml")

	assertIDs(t, repo.Candidates("banner"), "conv-legada")
}

func TestRepositoryLoadComponentSubstituiInventario(t *testing.T) {
	repo := testsupport.LoadInventory(t)
	if got := repo.Candidates("banner"); len(got) == 0 {
		t.Fatal("esperava candidatos antes da recarga")
	}

	testsupport.LoadComponentInto(t, repo, "banner", "empty/banner.yaml")

	if got := repo.Candidates("banner"); len(got) != 0 {
		t.Fatalf("recarga deveria substituir o inventario, recebeu %v", ids(got))
	}
	if got := repo.Candidates("card"); len(got) == 0 {
		t.Fatal("recarga de um componente nao pode afetar os demais")
	}
}

func TestRepositoryLeituraConcorrente(t *testing.T) {
	repo := testsupport.LoadInventory(t)

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				if got := repo.Candidates("banner"); len(got) != 4 {
					t.Errorf("esperava 4 candidatos, recebeu %d", len(got))
					return
				}
				if _, ok := repo.Fallback("banner", "veiculo"); !ok {
					t.Error("esperava fallback de veiculo")
					return
				}
			}
		}()
	}
	wg.Wait()
}
