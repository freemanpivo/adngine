package eligibility_test

import (
	"testing"

	"github.com/freemanpivo/adngine/internal/eligibility"
)

func bag() map[string]any {
	return map[string]any{
		"idcliente": "cliente-123",
		"score":     float64(700),
		"perfil": map[string]any{
			"faixa_renda": "A",
			"enderecos": []any{
				map[string]any{"uf": "SP", "tags": []any{"principal", "cobranca"}},
				map[string]any{"uf": "MG"},
			},
		},
		"produtos": []any{"veiculo", "eletrodomestico"},
		"nulo":     nil,
	}
}

func TestLookup(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		want  any
		found bool
	}{
		{name: "chave raiz", path: "idcliente", want: "cliente-123", found: true},
		{name: "mapa aninhado", path: "perfil.faixa_renda", want: "A", found: true},
		{name: "indice de lista", path: "produtos[0]", want: "veiculo", found: true},
		{name: "mapa dentro de lista", path: "perfil.enderecos[1].uf", want: "MG", found: true},
		{name: "lista dentro de lista", path: "perfil.enderecos[0].tags[1]", want: "cobranca", found: true},
		{name: "valor nulo existe", path: "nulo", want: nil, found: true},
		{name: "chave ausente", path: "inexistente", found: false},
		{name: "chave ausente em nivel intermediario", path: "perfil.nao_existe.uf", found: false},
		{name: "indice fora do range", path: "produtos[9]", found: false},
		{name: "indice negativo", path: "produtos[-1]", found: false},
		{name: "indice em quem nao e lista", path: "perfil[0]", found: false},
		{name: "campo em quem nao e mapa", path: "idcliente.uf", found: false},
		{name: "caminho vazio", path: "", found: false},
		{name: "segmento vazio", path: "perfil..faixa_renda", found: false},
		{name: "colchete nao fechado", path: "produtos[0", found: false},
		{name: "indice nao numerico", path: "produtos[primeiro]", found: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := eligibility.Lookup(bag(), tt.path)
			if found != tt.found {
				t.Fatalf("found = %v, esperava %v (valor %v)", found, tt.found, got)
			}
			if found && got != tt.want {
				t.Fatalf("valor = %v, esperava %v", got, tt.want)
			}
		})
	}
}

func TestLookupBagNula(t *testing.T) {
	if _, found := eligibility.Lookup(nil, "qualquer"); found {
		t.Fatal("bag nula nao deveria resolver nada")
	}
}
