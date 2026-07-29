package eligibility_test

import (
	"strings"
	"testing"

	"github.com/freemanpivo/adngine/internal/eligibility"
	"github.com/spf13/viper"
)

// ruleFromYAML passa pelo mesmo viper usado na carga do inventario, para que o
// teste veja exatamente a forma que o motor recebe em producao.
func ruleFromYAML(t testing.TB, doc string) map[string]any {
	t.Helper()

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(doc)); err != nil {
		t.Fatalf("lendo yaml: %v", err)
	}

	raw, ok := v.Get("rule").(map[string]any)
	if !ok {
		t.Fatalf("yaml nao produziu um mapa em rule: %T", v.Get("rule"))
	}
	return raw
}

const tresNiveis = `
rule:
  all:
    - field: perfil.faixa_renda
      op: in
      value: [A, B]
      weight: 2
    - any:
        - field: possui_veiculo
          op: eq
          value: false
          required: false
        - not:
            field: bloqueado
            op: truthy
            log_value: false
`

func TestParseRuleAninhada(t *testing.T) {
	root, err := eligibility.ParseRule(ruleFromYAML(t, tresNiveis))
	if err != nil {
		t.Fatalf("esperava parse valido, recebeu %v", err)
	}

	if root.Kind != eligibility.KindAll || len(root.Children) != 2 {
		t.Fatalf("raiz deveria ser um all com 2 filhos, recebeu %+v", root)
	}

	faixa := root.Children[0].Predicate
	if faixa == nil {
		t.Fatal("primeiro filho deveria ser predicado")
	}
	if faixa.Field != "perfil.faixa_renda" || faixa.Op != eligibility.OpIn {
		t.Fatalf("predicado inesperado: %+v", faixa)
	}
	if faixa.Weight != 2 {
		t.Errorf("weight = %v, esperava 2", faixa.Weight)
	}
	if !faixa.Required || !faixa.LogValue {
		t.Errorf("required e log_value deveriam ser true por default: %+v", faixa)
	}

	nivel2 := root.Children[1]
	if nivel2.Kind != eligibility.KindAny || len(nivel2.Children) != 2 {
		t.Fatalf("segundo filho deveria ser um any com 2 filhos, recebeu %+v", nivel2)
	}

	veiculo := nivel2.Children[0].Predicate
	if veiculo.Required {
		t.Error("required: false deveria ter sido lido")
	}
	if veiculo.Weight != 1 {
		t.Errorf("weight = %v, esperava o default 1", veiculo.Weight)
	}

	nivel3 := nivel2.Children[1]
	if nivel3.Kind != eligibility.KindNot || len(nivel3.Children) != 1 {
		t.Fatalf("terceiro nivel deveria ser um not com 1 filho, recebeu %+v", nivel3)
	}
	bloqueado := nivel3.Children[0].Predicate
	if bloqueado.Op != eligibility.OpTruthy || bloqueado.LogValue {
		t.Errorf("predicado do not inesperado: %+v", bloqueado)
	}
}

func TestParseRuleErros(t *testing.T) {
	tests := []struct {
		name   string
		doc    string
		wantIn string
	}{
		{
			name:   "no vazio",
			doc:    "rule:\n  all:\n    - {}\n",
			wantIn: "no vazio",
		},
		{
			name:   "dois combinadores no mesmo no",
			doc:    "rule:\n  all:\n    - field: a\n      op: eq\n      value: 1\n  any:\n    - field: b\n      op: eq\n      value: 1\n",
			wantIn: "mais de um combinador",
		},
		{
			name:   "combinador com campo extra",
			doc:    "rule:\n  all:\n    - field: a\n      op: eq\n      value: 1\n  field: b\n",
			wantIn: "nao aceita outros campos",
		},
		{
			name:   "all sem lista",
			doc:    "rule:\n  all:\n    field: a\n",
			wantIn: "espera uma lista",
		},
		{
			name:   "all vazio",
			doc:    "rule:\n  all: []\n",
			wantIn: "ao menos uma regra",
		},
		{
			name:   "campo desconhecido no predicado",
			doc:    "rule:\n  field: a\n  op: eq\n  value: 1\n  peso: 3\n",
			wantIn: "campo desconhecido",
		},
		{
			name:   "tipo errado em required",
			doc:    "rule:\n  field: a\n  op: eq\n  value: 1\n  required: talvez\n",
			wantIn: "esperava booleano",
		},
		{
			name:   "tipo errado em weight",
			doc:    "rule:\n  field: a\n  op: eq\n  value: 1\n  weight: muito\n",
			wantIn: "esperava numero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := eligibility.ParseRule(ruleFromYAML(t, tt.doc))
			if err == nil {
				t.Fatal("esperava erro, recebeu nil")
			}
			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Fatalf("erro deveria conter %q, recebeu %q", tt.wantIn, err)
			}
		})
	}
}

// Regra ausente nao e erro: equivale a static sem regra (RF-06).
func TestCompileRegraAusente(t *testing.T) {
	for _, raw := range []map[string]any{nil, {}} {
		compiled, err := eligibility.Compile("conv-1", raw)
		if err != nil {
			t.Fatalf("esperava nil, recebeu %v", err)
		}
		if compiled != nil {
			t.Fatalf("esperava regra nula, recebeu %+v", compiled)
		}
	}
}
