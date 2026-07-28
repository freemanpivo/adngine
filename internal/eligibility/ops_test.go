package eligibility_test

import (
	"testing"

	"github.com/freemanpivo/adngine/internal/eligibility"
)

// matches compila um predicado unico e devolve se ele deu match, que para uma
// regra de um predicado required equivale a elegibilidade.
func matches(t *testing.T, b map[string]any, predicate map[string]any) bool {
	t.Helper()

	compiled, err := eligibility.Compile("conv-teste", predicate)
	if err != nil {
		t.Fatalf("compilando %v: %v", predicate, err)
	}
	return eligibility.Evaluate(b, compiled, eligibility.Nop).Eligible
}

func predicate(field string, op eligibility.Op, value any) map[string]any {
	return map[string]any{"field": field, "op": string(op), "value": value}
}

func TestOperadores(t *testing.T) {
	tests := []struct {
		name      string
		predicate map[string]any
		want      bool
	}{
		{name: "eq hit", predicate: predicate("perfil.faixa_renda", eligibility.OpEq, "A"), want: true},
		{name: "eq miss", predicate: predicate("perfil.faixa_renda", eligibility.OpEq, "C"), want: false},
		{name: "eq campo ausente", predicate: predicate("nao_existe", eligibility.OpEq, "A"), want: false},

		{name: "ne hit", predicate: predicate("perfil.faixa_renda", eligibility.OpNe, "C"), want: true},
		{name: "ne miss", predicate: predicate("perfil.faixa_renda", eligibility.OpNe, "A"), want: false},
		{name: "ne campo ausente", predicate: predicate("nao_existe", eligibility.OpNe, "C"), want: false},

		{name: "gt hit", predicate: predicate("score", eligibility.OpGt, 500), want: true},
		{name: "gt miss", predicate: predicate("score", eligibility.OpGt, 700), want: false},
		{name: "gt campo ausente", predicate: predicate("nao_existe", eligibility.OpGt, 1), want: false},

		{name: "gte hit", predicate: predicate("score", eligibility.OpGte, 700), want: true},
		{name: "gte miss", predicate: predicate("score", eligibility.OpGte, 701), want: false},
		{name: "gte campo ausente", predicate: predicate("nao_existe", eligibility.OpGte, 1), want: false},

		{name: "lt hit", predicate: predicate("score", eligibility.OpLt, 900), want: true},
		{name: "lt miss", predicate: predicate("score", eligibility.OpLt, 700), want: false},
		{name: "lt campo ausente", predicate: predicate("nao_existe", eligibility.OpLt, 1), want: false},

		{name: "lte hit", predicate: predicate("score", eligibility.OpLte, 700), want: true},
		{name: "lte miss", predicate: predicate("score", eligibility.OpLte, 699), want: false},
		{name: "lte campo ausente", predicate: predicate("nao_existe", eligibility.OpLte, 1), want: false},

		{name: "in hit", predicate: predicate("perfil.faixa_renda", eligibility.OpIn, []any{"A", "B"}), want: true},
		{name: "in miss", predicate: predicate("perfil.faixa_renda", eligibility.OpIn, []any{"C", "D"}), want: false},
		{name: "in campo ausente", predicate: predicate("nao_existe", eligibility.OpIn, []any{"A"}), want: false},

		{name: "not_in hit", predicate: predicate("perfil.faixa_renda", eligibility.OpNotIn, []any{"C", "D"}), want: true},
		{name: "not_in miss", predicate: predicate("perfil.faixa_renda", eligibility.OpNotIn, []any{"A"}), want: false},
		{name: "not_in campo ausente", predicate: predicate("nao_existe", eligibility.OpNotIn, []any{"A"}), want: false},

		{name: "contains em lista hit", predicate: predicate("produtos", eligibility.OpContains, "veiculo"), want: true},
		{name: "contains em lista miss", predicate: predicate("produtos", eligibility.OpContains, "imovel"), want: false},
		{name: "contains em string hit", predicate: predicate("idcliente", eligibility.OpContains, "123"), want: true},
		{name: "contains em string miss", predicate: predicate("idcliente", eligibility.OpContains, "999"), want: false},
		{name: "contains campo ausente", predicate: predicate("nao_existe", eligibility.OpContains, "x"), want: false},

		{name: "exists hit", predicate: predicate("score", eligibility.OpExists, nil), want: true},
		{name: "exists em valor nulo", predicate: predicate("nulo", eligibility.OpExists, nil), want: true},
		{name: "exists campo ausente", predicate: predicate("nao_existe", eligibility.OpExists, nil), want: false},

		{name: "not_exists hit", predicate: predicate("nao_existe", eligibility.OpNotExists, nil), want: true},
		{name: "not_exists miss", predicate: predicate("score", eligibility.OpNotExists, nil), want: false},

		{name: "truthy hit", predicate: predicate("score", eligibility.OpTruthy, nil), want: true},
		{name: "truthy miss", predicate: predicate("nulo", eligibility.OpTruthy, nil), want: false},
		{name: "truthy campo ausente", predicate: predicate("nao_existe", eligibility.OpTruthy, nil), want: false},

		{name: "regex hit", predicate: predicate("idcliente", eligibility.OpRegex, `^cliente-\d+$`), want: true},
		{name: "regex miss", predicate: predicate("perfil.faixa_renda", eligibility.OpRegex, `^\d+$`), want: false},
		{name: "regex campo ausente", predicate: predicate("nao_existe", eligibility.OpRegex, `.*`), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matches(t, bag(), tt.predicate); got != tt.want {
				t.Fatalf("match = %v, esperava %v", got, tt.want)
			}
		})
	}
}

// Operador que espera lista ou string aplicado a um tipo incompativel reprova
// sem panic.
func TestOperadoresComTiposIncompativeis(t *testing.T) {
	incompativel := map[string]any{
		"mapa":  map[string]any{"a": 1},
		"lista": []any{1, 2},
	}

	tests := []map[string]any{
		predicate("mapa", eligibility.OpGt, 1),
		predicate("mapa", eligibility.OpContains, "a"),
		predicate("lista", eligibility.OpEq, "1,2"),
		predicate("mapa", eligibility.OpRegex, ".*"),
		predicate("lista", eligibility.OpLte, 10),
	}

	for _, p := range tests {
		t.Run(p["field"].(string)+" "+p["op"].(string), func(t *testing.T) {
			if matches(t, incompativel, p) {
				t.Fatal("tipo incomparavel nao deveria dar match")
			}
		})
	}
}
