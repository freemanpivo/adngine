package eligibility_test

import (
	"encoding/json"
	"testing"

	"github.com/freemanpivo/adngine/internal/eligibility"
)

// O mesmo YAML precisa funcionar com numero vindo do DynamoDB (string) e de JSON
// (float64), sem que o analista saiba de onde veio o dado (RF-10).
func TestComparacaoToleranteATipo(t *testing.T) {
	tests := []struct {
		name      string
		bag       map[string]any
		predicate map[string]any
		want      bool
	}{
		{
			name:      "numero como string do dynamodb",
			bag:       map[string]any{"score": "700"},
			predicate: predicate("score", eligibility.OpEq, 700),
			want:      true,
		},
		{
			name:      "numero como float64 do json",
			bag:       map[string]any{"score": float64(700)},
			predicate: predicate("score", eligibility.OpEq, 700),
			want:      true,
		},
		{
			name:      "json.Number",
			bag:       map[string]any{"score": json.Number("700")},
			predicate: predicate("score", eligibility.OpGte, 700),
			want:      true,
		},
		{
			name:      "numero com espacos",
			bag:       map[string]any{"score": " 700 "},
			predicate: predicate("score", eligibility.OpEq, 700),
			want:      true,
		},
		{
			name:      "int contra float",
			bag:       map[string]any{"score": 700},
			predicate: predicate("score", eligibility.OpEq, 700.0),
			want:      true,
		},
		{
			name:      "comparacao numerica com valor textual",
			bag:       map[string]any{"score": "650"},
			predicate: predicate("score", eligibility.OpLt, "700"),
			want:      true,
		},
		{
			name:      "string nao numerica compara lexicograficamente",
			bag:       map[string]any{"faixa": "B"},
			predicate: predicate("faixa", eligibility.OpGt, "A"),
			want:      true,
		},
		{
			name:      "booleano contra string",
			bag:       map[string]any{"possui_veiculo": "false"},
			predicate: predicate("possui_veiculo", eligibility.OpEq, false),
			want:      true,
		},
		{
			name:      "booleano contra booleano",
			bag:       map[string]any{"possui_veiculo": true},
			predicate: predicate("possui_veiculo", eligibility.OpEq, true),
			want:      true,
		},
		{
			name:      "numero contra booleano nao compara",
			bag:       map[string]any{"possui_veiculo": 1},
			predicate: predicate("possui_veiculo", eligibility.OpEq, true),
			want:      false,
		},
		{
			name:      "in com numero textual",
			bag:       map[string]any{"score": "700"},
			predicate: predicate("score", eligibility.OpIn, []any{600, 700}),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matches(t, tt.bag, tt.predicate); got != tt.want {
				t.Fatalf("match = %v, esperava %v", got, tt.want)
			}
		})
	}
}

// truthy e mais permissivo que a conversao para booleano: flags textuais como
// "S" contam como verdadeiras, mas literais falsos nao.
func TestTruthy(t *testing.T) {
	tests := []struct {
		value any
		want  bool
	}{
		{value: "S", want: true},
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "sim", want: true},
		{value: true, want: true},
		{value: float64(1), want: true},
		{value: []any{"a"}, want: true},
		{value: "false", want: false},
		{value: "FALSE", want: false},
		{value: "0", want: false},
		{value: "nao", want: false},
		{value: "", want: false},
		{value: "   ", want: false},
		{value: false, want: false},
		{value: float64(0), want: false},
		{value: nil, want: false},
		{value: []any{}, want: false},
	}

	for _, tt := range tests {
		t.Run(renderName(tt.value), func(t *testing.T) {
			b := map[string]any{"flag": tt.value}
			if got := matches(t, b, predicate("flag", eligibility.OpTruthy, nil)); got != tt.want {
				t.Fatalf("truthy(%#v) = %v, esperava %v", tt.value, got, tt.want)
			}
		})
	}
}

func renderName(v any) string {
	if s, ok := v.(string); ok {
		return "string:" + s
	}
	return "valor"
}
