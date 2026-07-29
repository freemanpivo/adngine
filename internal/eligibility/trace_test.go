package eligibility_test

import (
	"strings"
	"testing"

	"github.com/freemanpivo/adngine/internal/eligibility"
)

func TestRecorderRegistraTodosOsPredicados(t *testing.T) {
	rec := eligibility.NewRecorder()
	got := eligibility.Evaluate(bag(), compile(t, regraCompleta), rec)

	if !got.Eligible {
		t.Fatalf("esperava conversa elegivel, recebeu %+v", got)
	}

	wantLabels := []string{"perfil.faixa_renda in [A,B]", "score gte 700", "bloqueado truthy"}
	if len(rec.Predicates) != len(wantLabels) {
		t.Fatalf("trace com %d predicados, esperava %d", len(rec.Predicates), len(wantLabels))
	}
	for i, want := range wantLabels {
		if rec.Predicates[i].Label != want {
			t.Errorf("trace[%d].Label = %q, esperava %q", i, rec.Predicates[i].Label, want)
		}
		if rec.Predicates[i].PredicateID == "" {
			t.Errorf("trace[%d] sem predicate_id", i)
		}
	}

	if rec.Predicates[0].Actual != "A" || !rec.Predicates[0].Matched {
		t.Errorf("primeiro predicado deveria registrar match com actual A: %+v", rec.Predicates[0])
	}
	if rec.Predicates[2].Found {
		t.Errorf("campo ausente deveria ficar marcado como nao encontrado: %+v", rec.Predicates[2])
	}
}

// Um any nao curto-circuita: o trace precisa mostrar os predicados que nem
// decidiram a elegibilidade, senao a pergunta "por que a conversa X nao
// apareceu" fica sem resposta.
func TestRecorderNaoCurtoCircuita(t *testing.T) {
	doc := `
rule:
  any:
    - field: perfil.faixa_renda
      op: eq
      value: A
    - field: score
      op: gte
      value: 900
`
	rec := eligibility.NewRecorder()
	eligibility.Evaluate(bag(), compile(t, doc), rec)

	if len(rec.Predicates) != 2 {
		t.Fatalf("trace com %d predicados, esperava 2", len(rec.Predicates))
	}
	if !rec.Predicates[0].Matched || rec.Predicates[1].Matched {
		t.Fatalf("trace inesperado: %+v", rec.Predicates)
	}
}

func TestRecorderRespeitaLogValue(t *testing.T) {
	doc := `
rule:
  all:
    - field: idcliente
      op: exists
      log_value: false
    - field: perfil.faixa_renda
      op: eq
      value: A
`
	rec := eligibility.NewRecorder()
	eligibility.Evaluate(bag(), compile(t, doc), rec)

	if rec.Predicates[0].Actual != "" {
		t.Errorf("log_value: false nao deveria registrar o valor, recebeu %q", rec.Predicates[0].Actual)
	}
	if rec.Predicates[1].Actual != "A" {
		t.Errorf("log_value: true deveria registrar o valor, recebeu %q", rec.Predicates[1].Actual)
	}
}

func TestRecorderTruncaValor(t *testing.T) {
	longo := strings.Repeat("x", 200)
	doc := `
rule:
  field: observacao
  op: exists
`
	rec := eligibility.NewRecorder()
	eligibility.Evaluate(map[string]any{"observacao": longo}, compile(t, doc), rec)

	if got := len([]rune(rec.Predicates[0].Actual)); got != eligibility.MaxActualLen {
		t.Fatalf("actual com %d caracteres, esperava %d", got, eligibility.MaxActualLen)
	}
}

// fields_read denuncia regra apontando para chave que nao existe na bag.
func TestEvaluateFieldsRead(t *testing.T) {
	doc := `
rule:
  all:
    - field: score
      op: gte
      value: 700
    - field: coluna_que_nao_existe
      op: exists
      required: false
    - field: score
      op: lte
      value: 900
`
	got := eligibility.Evaluate(bag(), compile(t, doc), eligibility.NewRecorder())

	want := []string{"score", "coluna_que_nao_existe"}
	if len(got.FieldsRead) != len(want) {
		t.Fatalf("FieldsRead = %v, esperava %v", got.FieldsRead, want)
	}
	for i := range want {
		if got.FieldsRead[i] != want[i] {
			t.Fatalf("FieldsRead = %v, esperava %v", got.FieldsRead, want)
		}
	}
}

// RF-37: com o trace desligado a avaliacao nao pode alocar nada.
func TestEvaluateColetorNuloNaoAloca(t *testing.T) {
	compiled := compile(t, regraCompleta)
	b := bag()

	allocs := testing.AllocsPerRun(100, func() {
		eligibility.Evaluate(b, compiled, eligibility.Nop)
	})
	if allocs != 0 {
		t.Fatalf("avaliacao com coletor nulo alocou %v vezes", allocs)
	}
}

func BenchmarkEvaluateColetorNulo(b *testing.B) {
	compiled := compile(b, regraCompleta)
	data := bag()

	b.ReportAllocs()
	for b.Loop() {
		eligibility.Evaluate(data, compiled, eligibility.Nop)
	}
}

func BenchmarkEvaluateComTrace(b *testing.B) {
	compiled := compile(b, regraCompleta)
	data := bag()

	b.ReportAllocs()
	for b.Loop() {
		eligibility.Evaluate(data, compiled, eligibility.NewRecorder())
	}
}
