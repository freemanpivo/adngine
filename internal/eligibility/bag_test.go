package eligibility_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/freemanpivo/adngine/internal/eligibility"
)

// A bag nao tem schema nem decoder unico: DynamoDB, JSON e os atributos vindos
// do request produzem formas diferentes para o mesmo dado.
func TestBagComFormasAlternativas(t *testing.T) {
	stringer, err := url.Parse("https://example.com/perfil")
	if err != nil {
		t.Fatalf("montando stringer: %v", err)
	}

	b := map[string]any{
		"perfil": map[any]any{
			"faixa_renda": "A",
			"enderecos":   []string{"SP", "MG"},
		},
		"idade":     int8(30),
		"limite":    uint16(5000),
		"parcelas":  int64(12),
		"desconto":  float32(0.5),
		"url":       stringer,
		"documento": "12345678900",
	}

	tests := []struct {
		name      string
		predicate map[string]any
		want      bool
	}{
		{name: "mapa com chave nao tipada", predicate: predicate("perfil.faixa_renda", eligibility.OpEq, "A"), want: true},
		{name: "lista de string", predicate: predicate("perfil.enderecos[1]", eligibility.OpEq, "MG"), want: true},
		{name: "lista de string fora do range", predicate: predicate("perfil.enderecos[5]", eligibility.OpExists, nil), want: false},
		{name: "contains em lista de string", predicate: predicate("perfil.enderecos", eligibility.OpContains, "SP"), want: true},
		{name: "inteiro de 8 bits", predicate: predicate("idade", eligibility.OpGte, 18), want: true},
		{name: "inteiro sem sinal", predicate: predicate("limite", eligibility.OpEq, 5000), want: true},
		{name: "inteiro de 64 bits", predicate: predicate("parcelas", eligibility.OpIn, []any{6, 12, 24}), want: true},
		{name: "float de 32 bits", predicate: predicate("desconto", eligibility.OpLt, 1), want: true},
		{name: "valor com String()", predicate: predicate("url", eligibility.OpContains, "example.com"), want: true},
		{name: "regex sobre texto", predicate: predicate("documento", eligibility.OpRegex, `^\d{11}$`), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matches(t, b, tt.predicate); got != tt.want {
				t.Fatalf("match = %v, esperava %v", got, tt.want)
			}
		})
	}
}

// Regra com todos os pesos zerados nao pode dividir por zero.
func TestAderenciaComPesosZerados(t *testing.T) {
	doc := `
rule:
  all:
    - field: perfil.faixa_renda
      op: eq
      value: A
      weight: 0
`
	got := eligibility.Evaluate(bag(), compile(t, doc), eligibility.Nop)

	if !got.Eligible || got.Adherence != 1 {
		t.Fatalf("esperava {true, 1.0}, recebeu %+v", got)
	}
}

func TestParseRuleFilhoQueNaoEMapa(t *testing.T) {
	_, err := eligibility.ParseRule(ruleFromYAML(t, "rule:\n  all:\n    - apenas texto\n"))
	if err == nil {
		t.Fatal("esperava erro, recebeu nil")
	}
	if !strings.Contains(err.Error(), "esperava um mapa") {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func TestParseRuleOperadorNaoTextual(t *testing.T) {
	_, err := eligibility.ParseRule(ruleFromYAML(t, "rule:\n  field: score\n  op: 10\n  value: 1\n"))
	if err == nil {
		t.Fatal("esperava erro, recebeu nil")
	}
	if !strings.Contains(err.Error(), "esperava string") {
		t.Fatalf("erro inesperado: %v", err)
	}
}

// O label vai para o log e para a mensagem de erro: precisa aguentar qualquer
// valor vindo do YAML.
func TestLabelComValoresIncomuns(t *testing.T) {
	doc := `
rule:
  all:
    - field: nulo
      op: eq
      value: ~
    - field: score
      op: in
      value: [700, true]
`
	compiled := compile(t, doc)

	wantLabels := []string{"nulo eq null", "score in [700,true]"}
	for i, want := range wantLabels {
		if compiled.Predicates[i].Label != want {
			t.Errorf("Predicates[%d].Label = %q, esperava %q", i, compiled.Predicates[i].Label, want)
		}
	}
}
