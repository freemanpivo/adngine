package eligibility_test

import (
	"testing"

	"github.com/freemanpivo/adngine/internal/eligibility"
)

func compile(t testing.TB, doc string) *eligibility.CompiledRule {
	t.Helper()

	compiled, err := eligibility.Compile("conv-teste", ruleFromYAML(t, doc))
	if err != nil {
		t.Fatalf("compilando regra: %v", err)
	}
	return compiled
}

func TestEvaluateRegraAusente(t *testing.T) {
	got := eligibility.Evaluate(bag(), nil, eligibility.Nop)

	if !got.Eligible || got.Adherence != 1 {
		t.Fatalf("regra ausente deveria ser {true, 1.0}, recebeu %+v", got)
	}
}

func TestEvaluateElegibilidadeEAderencia(t *testing.T) {
	tests := []struct {
		name          string
		doc           string
		bag           map[string]any
		wantEligible  bool
		wantAdherence float64
	}{
		{
			name: "all satisfeito",
			doc: `
rule:
  all:
    - field: score
      op: gte
      value: 700
    - field: perfil.faixa_renda
      op: in
      value: [A, B]
`,
			bag:           bag(),
			wantEligible:  true,
			wantAdherence: 1,
		},
		{
			name: "predicado required reprovado descarta mas a aderencia continua sendo calculada",
			doc: `
rule:
  all:
    - field: score
      op: gte
      value: 900
      weight: 3
    - field: perfil.faixa_renda
      op: in
      value: [A, B]
      weight: 1
`,
			bag:           bag(),
			wantEligible:  false,
			wantAdherence: 0.25,
		},
		{
			name: "predicado opcional nunca desqualifica",
			doc: `
rule:
  all:
    - field: score
      op: gte
      value: 700
    - field: possui_veiculo
      op: eq
      value: true
      required: false
`,
			bag:           bag(),
			wantEligible:  true,
			wantAdherence: 0.5,
		},
		{
			name: "regra so com predicados opcionais e sempre elegivel",
			doc: `
rule:
  all:
    - field: nao_existe
      op: exists
      required: false
    - field: score
      op: gte
      value: 900
      required: false
`,
			bag:           bag(),
			wantEligible:  true,
			wantAdherence: 0,
		},
		{
			name: "any com um ramo satisfeito",
			doc: `
rule:
  any:
    - field: perfil.faixa_renda
      op: eq
      value: C
    - field: score
      op: gte
      value: 700
`,
			bag:           bag(),
			wantEligible:  true,
			wantAdherence: 0.5,
		},
		{
			name: "any sem nenhum ramo satisfeito",
			doc: `
rule:
  any:
    - field: perfil.faixa_renda
      op: eq
      value: C
    - field: score
      op: gte
      value: 900
`,
			bag:           bag(),
			wantEligible:  false,
			wantAdherence: 0,
		},
		{
			name: "not satisfeito conta o peso do predicado que nao deu match",
			doc: `
rule:
  not:
    field: bloqueado
    op: truthy
`,
			bag:           bag(),
			wantEligible:  true,
			wantAdherence: 1,
		},
		{
			name: "not reprovado zera a aderencia",
			doc: `
rule:
  not:
    field: score
    op: gte
    value: 700
`,
			bag:           bag(),
			wantEligible:  false,
			wantAdherence: 0,
		},
		{
			name: "opcional dentro de not permanece transparente",
			doc: `
rule:
  all:
    - field: score
      op: gte
      value: 700
    - not:
        field: perfil.faixa_renda
        op: eq
        value: A
        required: false
`,
			bag:           bag(),
			wantEligible:  true,
			wantAdherence: 0.5,
		},
		{
			name: "arvore de tres niveis",
			doc: `
rule:
  all:
    - field: score
      op: gte
      value: 700
      weight: 2
    - any:
        - field: perfil.faixa_renda
          op: eq
          value: C
        - not:
            field: produtos
            op: contains
            value: imovel
`,
			bag:           bag(),
			wantEligible:  true,
			wantAdherence: 0.75,
		},
		{
			name: "bag vazia reprova tudo menos not_exists",
			doc: `
rule:
  all:
    - field: score
      op: gte
      value: 700
    - field: bloqueado
      op: not_exists
`,
			bag:           map[string]any{},
			wantEligible:  false,
			wantAdherence: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eligibility.Evaluate(tt.bag, compile(t, tt.doc), eligibility.Nop)

			if got.Eligible != tt.wantEligible {
				t.Errorf("Eligible = %v, esperava %v", got.Eligible, tt.wantEligible)
			}
			if got.Adherence != tt.wantAdherence {
				t.Errorf("Adherence = %v, esperava %v", got.Adherence, tt.wantAdherence)
			}
		})
	}
}

func TestEvaluatePrimeiroPredicadoReprovado(t *testing.T) {
	doc := `
rule:
  all:
    - field: perfil.faixa_renda
      op: in
      value: [A, B]
    - field: score
      op: gte
      value: 900
    - field: bloqueado
      op: exists
`
	got := eligibility.Evaluate(bag(), compile(t, doc), eligibility.Nop)

	if got.Eligible {
		t.Fatal("esperava conversa nao elegivel")
	}
	if got.FirstFailed == nil {
		t.Fatal("esperava o primeiro predicado reprovado")
	}
	if got.FirstFailed.Label != "score gte 900" {
		t.Fatalf("FirstFailed = %q, esperava o predicado de score", got.FirstFailed.Label)
	}
	if got.FirstFailed.ID != "conv-teste#1" {
		t.Fatalf("FirstFailed.ID = %q, esperava conv-teste#1", got.FirstFailed.ID)
	}
}

// FirstFailed so faz sentido quando a conversa foi descartada: um predicado
// reprovado dentro de um any vencedor nao e motivo de exclusao.
func TestEvaluateSemPredicadoReprovadoQuandoElegivel(t *testing.T) {
	doc := `
rule:
  any:
    - field: score
      op: gte
      value: 900
    - field: perfil.faixa_renda
      op: eq
      value: A
`
	got := eligibility.Evaluate(bag(), compile(t, doc), eligibility.Nop)

	if !got.Eligible {
		t.Fatal("esperava conversa elegivel")
	}
	if got.FirstFailed != nil {
		t.Fatalf("esperava FirstFailed nulo, recebeu %q", got.FirstFailed.Label)
	}
}

func TestEvaluateColetorNulo(t *testing.T) {
	doc := `
rule:
  field: score
  op: gte
  value: 700
`
	compiled := compile(t, doc)

	if got := eligibility.Evaluate(bag(), compiled, eligibility.Nop); got.FieldsRead != nil {
		t.Fatalf("coletor nulo nao deveria montar FieldsRead, recebeu %v", got.FieldsRead)
	}
	if got := eligibility.Evaluate(bag(), compiled, nil); !got.Eligible {
		t.Fatal("coletor nil deveria cair no coletor nulo")
	}
}
