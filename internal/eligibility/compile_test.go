package eligibility_test

import (
	"strings"
	"testing"

	"github.com/freemanpivo/adngine/internal/eligibility"
)

const regraCompleta = `
rule:
  all:
    - field: perfil.faixa_renda
      op: in
      value: [A, B]
      weight: 2
    - any:
        - field: score
          op: gte
          value: 700
        - not:
            field: bloqueado
            op: truthy
            weight: 0.5
`

func TestCompilePreComputa(t *testing.T) {
	compiled := compile(t, regraCompleta)

	if got := len(compiled.Predicates); got != 3 {
		t.Fatalf("lista plana com %d predicados, esperava 3", got)
	}
	if compiled.TotalWeight != 3.5 {
		t.Errorf("TotalWeight = %v, esperava 3.5", compiled.TotalWeight)
	}

	wantIDs := []string{"conv-teste#0", "conv-teste#1", "conv-teste#2"}
	wantLabels := []string{"perfil.faixa_renda in [A,B]", "score gte 700", "bloqueado truthy"}
	for i, p := range compiled.Predicates {
		if p.ID != wantIDs[i] {
			t.Errorf("Predicates[%d].ID = %q, esperava %q", i, p.ID, wantIDs[i])
		}
		if p.Label != wantLabels[i] {
			t.Errorf("Predicates[%d].Label = %q, esperava %q", i, p.Label, wantLabels[i])
		}
	}
}

// O predicate_id vira label de metrica e chave de busca no log: precisa ser o
// mesmo entre reinicios enquanto o arquivo nao mudar.
func TestCompileIDsEstaveis(t *testing.T) {
	primeira := compile(t, regraCompleta)
	segunda := compile(t, regraCompleta)

	for i := range primeira.Predicates {
		if primeira.Predicates[i].ID != segunda.Predicates[i].ID {
			t.Fatalf("id instavel: %q vs %q", primeira.Predicates[i].ID, segunda.Predicates[i].ID)
		}
		if primeira.Predicates[i].Label != segunda.Predicates[i].Label {
			t.Fatalf("label instavel: %q vs %q", primeira.Predicates[i].Label, segunda.Predicates[i].Label)
		}
	}
}

func TestCompileErros(t *testing.T) {
	tests := []struct {
		name   string
		doc    string
		wantIn string
	}{
		{
			name:   "operador desconhecido",
			doc:    "rule:\n  field: score\n  op: aproximadamente\n  value: 700\n",
			wantIn: `operador desconhecido "aproximadamente"`,
		},
		{
			name:   "operador vazio",
			doc:    "rule:\n  field: score\n  value: 700\n",
			wantIn: "operador desconhecido",
		},
		{
			name:   "field vazio",
			doc:    "rule:\n  field: \"\"\n  op: eq\n  value: 700\n",
			wantIn: "field e obrigatorio",
		},
		{
			name:   "field ausente",
			doc:    "rule:\n  op: eq\n  value: 700\n",
			wantIn: "field e obrigatorio",
		},
		{
			name:   "in sem lista",
			doc:    "rule:\n  field: score\n  op: in\n  value: 700\n",
			wantIn: `operador "in" exige uma lista em value`,
		},
		{
			name:   "not_in sem lista",
			doc:    "rule:\n  field: score\n  op: not_in\n  value: 700\n",
			wantIn: `operador "not_in" exige uma lista em value`,
		},
		{
			name:   "regex invalida",
			doc:    "rule:\n  field: documento\n  op: regex\n  value: \"([0-9]\"\n",
			wantIn: "regex invalida",
		},
		{
			name:   "regex com valor nao textual",
			doc:    "rule:\n  field: documento\n  op: regex\n  value: 10\n",
			wantIn: "exige uma string em value",
		},
		{
			name:   "weight negativo",
			doc:    "rule:\n  field: score\n  op: eq\n  value: 700\n  weight: -1\n",
			wantIn: "weight nao pode ser negativo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eligibility.ValidateRule(ruleFromYAML(t, tt.doc))
			if err == nil {
				t.Fatal("esperava erro, recebeu nil")
			}
			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Fatalf("erro deveria conter %q, recebeu %q", tt.wantIn, err)
			}
		})
	}
}

// O erro precisa apontar qual predicado quebrou, senao uma regra grande vira
// caca ao tesouro.
func TestValidateRuleNomeiaOPredicado(t *testing.T) {
	doc := `
rule:
  all:
    - field: score
      op: gte
      value: 700
    - field: documento
      op: regex
      value: "([0-9]"
`
	err := eligibility.ValidateRule(ruleFromYAML(t, doc))
	if err == nil {
		t.Fatal("esperava erro, recebeu nil")
	}
	if !strings.Contains(err.Error(), "documento regex") {
		t.Fatalf("erro deveria citar o predicado, recebeu %q", err)
	}
}

func TestValidateRuleAceitaRegraValida(t *testing.T) {
	if err := eligibility.ValidateRule(ruleFromYAML(t, regraCompleta)); err != nil {
		t.Fatalf("esperava regra valida, recebeu %v", err)
	}
}
