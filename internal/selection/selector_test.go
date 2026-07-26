package selection_test

import (
	"testing"

	"github.com/freemanpivo/adngine/internal/conversation"
	"github.com/freemanpivo/adngine/internal/selection"
	"github.com/freemanpivo/adngine/internal/testsupport"
)

func conv(id, product string, priority int) conversation.Conversation {
	return conversation.Conversation{
		ID:       id,
		Type:     conversation.TypeAction,
		Product:  product,
		Text:     "texto de " + id,
		Link:     "https://example.com/" + id,
		Priority: priority,
	}
}

func selectors() map[string]selection.Selector {
	return map[string]selection.Selector{
		selection.ComponentBanner: selection.NewBannerSelector(),
		selection.ComponentCard:   selection.NewCardSelector(),
		selection.ComponentFooter: selection.NewFooterSelector(),
	}
}

// Enquanto os tres componentes delegam ao mesmo bestMatch, a mesma tabela vale
// para todos. Quando um deles ganhar regra propria, o teste dele sai daqui.
func TestSelectorsCompartilhamORanking(t *testing.T) {
	tests := []struct {
		name       string
		client     conversation.Client
		candidates []conversation.Conversation
		want       string
	}{
		{
			name:       "sem candidatos",
			client:     conversation.Client{ID: "c1", Product: "veiculo"},
			candidates: nil,
			want:       "",
		},
		{
			name:   "maior prioridade vence",
			client: conversation.Client{ID: "c1", Product: "veiculo"},
			candidates: []conversation.Conversation{
				conv("baixa", "veiculo", 1),
				conv("alta", "veiculo", 10),
				conv("media", "veiculo", 5),
			},
			want: "alta",
		},
		{
			name:   "conversa de outro produto e descartada mesmo com prioridade maior",
			client: conversation.Client{ID: "c1", Product: "veiculo"},
			candidates: []conversation.Conversation{
				conv("eletro", "eletrodomestico", 99),
				conv("veiculo", "veiculo", 1),
			},
			want: "veiculo",
		},
		{
			name:   "conversa sem produto e elegivel em qualquer contexto",
			client: conversation.Client{ID: "c1", Product: "veiculo"},
			candidates: []conversation.Conversation{
				conv("generica", "", 10),
				conv("veiculo", "veiculo", 5),
			},
			want: "generica",
		},
		{
			name:   "cliente sem produto so recebe conversa sem produto",
			client: conversation.Client{ID: "c1"},
			candidates: []conversation.Conversation{
				conv("veiculo", "veiculo", 99),
				conv("eletro", "eletrodomestico", 98),
				conv("generica", "", 1),
			},
			want: "generica",
		},
		{
			name:   "cliente sem produto e sem conversa generica nao recebe nada",
			client: conversation.Client{ID: "c1"},
			candidates: []conversation.Conversation{
				conv("veiculo", "veiculo", 99),
			},
			want: "",
		},
		{
			name:   "produto e comparado por igualdade exata",
			client: conversation.Client{ID: "c1", Product: "Veiculo"},
			candidates: []conversation.Conversation{
				conv("veiculo", "veiculo", 10),
			},
			want: "",
		},
	}

	for _, tt := range tests {
		for component, selector := range selectors() {
			t.Run(component+"/"+tt.name, func(t *testing.T) {
				got, ok := selector.Select(tt.client, tt.candidates)

				if tt.want == "" {
					if ok {
						t.Fatalf("esperava nenhuma selecao, recebeu %s", got.ID)
					}
					if got != nil {
						t.Fatalf("esperava nil quando ok e falso, recebeu %v", got)
					}
					return
				}
				if !ok {
					t.Fatalf("esperava %s, nao houve selecao", tt.want)
				}
				if got.ID != tt.want {
					t.Fatalf("esperava %s, recebeu %s", tt.want, got.ID)
				}
			})
		}
	}
}

// Comportamento atual: o empate de prioridade e resolvido pela ordem do arquivo.
// A RF-13 troca isso por desempate deterministico via id, e este teste e o
// alarme de que a troca aconteceu.
func TestSelectorEmpateDePrioridadeMantemAOrdemDeEntrada(t *testing.T) {
	candidates := []conversation.Conversation{
		conv("primeira", "", 7),
		conv("segunda", "", 7),
	}

	got, ok := selection.NewBannerSelector().Select(conversation.Client{ID: "c1"}, candidates)
	if !ok {
		t.Fatal("esperava uma selecao")
	}
	if got.ID != "primeira" {
		t.Fatalf("esperava primeira, recebeu %s", got.ID)
	}
}

// O selector devolve um ponteiro; quem consome nao pode conseguir corromper o
// inventario em memoria atraves dele.
func TestRegistrySelectNaoExpoeOInventarioParaEscrita(t *testing.T) {
	registry := selection.NewRegistry(testsupport.LoadRepository(t, "inventory_valid.yaml"))
	client := conversation.Client{ID: "c1", Product: "veiculo"}

	first, ok := registry.Select(selection.ComponentBanner, client)
	if !ok {
		t.Fatal("esperava uma selecao")
	}
	first.Priority = -1
	first.Text = "corrompido"

	second, ok := registry.Select(selection.ComponentBanner, client)
	if !ok {
		t.Fatal("esperava uma selecao na segunda chamada")
	}
	if second.Priority != 10 || second.Text == "corrompido" {
		t.Fatalf("inventario corrompido pelo consumidor: %+v", second)
	}
}

func TestRegistrySelect(t *testing.T) {
	repo := testsupport.LoadRepository(t, "inventory_valid.yaml")
	registry := selection.NewRegistry(repo)

	tests := []struct {
		name      string
		component string
		client    conversation.Client
		want      string
	}{
		{
			name:      "banner com produto veiculo",
			component: selection.ComponentBanner,
			client:    conversation.Client{ID: "c1", Product: "veiculo"},
			want:      "conv-veiculo-alta",
		},
		{
			name:      "banner com produto eletrodomestico",
			component: selection.ComponentBanner,
			client:    conversation.Client{ID: "c1", Product: "eletrodomestico"},
			want:      "conv-eletro-alta",
		},
		{
			name:      "banner sem produto cai na conversa generica",
			component: selection.ComponentBanner,
			client:    conversation.Client{ID: "c1"},
			want:      "conv-generica",
		},
		{
			name:      "card nao enxerga o inventario exclusivo de banner",
			component: selection.ComponentCard,
			client:    conversation.Client{ID: "c1", Product: "eletrodomestico"},
			want:      "conv-generica",
		},
		{
			name:      "footer",
			component: selection.ComponentFooter,
			client:    conversation.Client{ID: "c1", Product: "veiculo"},
			want:      "conv-empate-a",
		},
		{
			name:      "componente nao registrado",
			component: "sidebar",
			client:    conversation.Client{ID: "c1", Product: "veiculo"},
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := registry.Select(tt.component, tt.client)

			if tt.want == "" {
				if ok {
					t.Fatalf("esperava nenhuma selecao, recebeu %s", got.ID)
				}
				return
			}
			if !ok {
				t.Fatalf("esperava %s, nao houve selecao", tt.want)
			}
			if got.ID != tt.want {
				t.Fatalf("esperava %s, recebeu %s", tt.want, got.ID)
			}
		})
	}
}

func TestRegistrySelectComInventarioVazio(t *testing.T) {
	registry := selection.NewRegistry(testsupport.LoadRepository(t, "inventory_empty.yaml"))

	if _, ok := registry.Select(selection.ComponentBanner, conversation.Client{ID: "c1"}); ok {
		t.Fatal("esperava nenhuma selecao com inventario vazio")
	}
}
