package eligibility

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CompiledRule e a regra pronta para avaliacao. Predicates e TotalWeight sao
// derivados uma unica vez na carga do inventario (RF-35): a avaliacao percorre a
// arvore, e a lista plana existe so para o trace e para o denominador da
// aderencia.
type CompiledRule struct {
	Root        Rule
	Predicates  []*Predicate
	TotalWeight float64
}

// Compile parseia e pre-computa a regra de uma conversa. conversationID entra no
// predicate_id, entao o id e estavel entre reinicios enquanto o arquivo nao
// mudar.
func Compile(conversationID string, raw map[string]any) (*CompiledRule, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	root, err := ParseRule(raw)
	if err != nil {
		return nil, err
	}

	compiled := &CompiledRule{Root: root}
	var errs []error
	compiled.walk(&compiled.Root, conversationID, &errs)
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return compiled, nil
}

func (c *CompiledRule) walk(rule *Rule, conversationID string, errs *[]error) {
	if rule.Kind != KindPredicate {
		for i := range rule.Children {
			c.walk(&rule.Children[i], conversationID, errs)
		}
		return
	}

	p := rule.Predicate
	p.ID = fmt.Sprintf("%s#%d", conversationID, len(c.Predicates))
	p.Label = renderLabel(p)

	if err := prepare(p); err != nil {
		*errs = append(*errs, fmt.Errorf("%s (%s): %w", p.ID, p.Label, err))
	}

	c.Predicates = append(c.Predicates, p)
	c.TotalWeight += p.Weight
}

// prepare valida o predicado e compila o que so pode falhar aqui, nunca durante
// a avaliacao.
func prepare(p *Predicate) error {
	var errs []error

	if p.Field == "" {
		errs = append(errs, errors.New("field e obrigatorio"))
	}
	if !knownOp(p.Op) {
		errs = append(errs, fmt.Errorf("operador desconhecido %q", p.Op))
	}
	if p.Weight < 0 {
		errs = append(errs, fmt.Errorf("weight nao pode ser negativo: %v", p.Weight))
	}
	if listOperators[p.Op] {
		// Normaliza aqui para que a avaliacao percorra sempre um []any pronto.
		list, ok := toList(p.Value)
		if !ok {
			errs = append(errs, fmt.Errorf("operador %q exige uma lista em value", p.Op))
		} else {
			p.Value = list
		}
	}
	if p.Op == OpRegex {
		pattern, ok := p.Value.(string)
		if !ok {
			errs = append(errs, fmt.Errorf("operador %q exige uma string em value", OpRegex))
		} else {
			re, err := regexp.Compile(pattern)
			if err != nil {
				errs = append(errs, fmt.Errorf("regex invalida: %w", err))
			}
			p.regex = re
		}
	}

	return errors.Join(errs...)
}

// renderLabel produz a forma legivel usada no trace e nas mensagens de erro,
// por exemplo "perfil.faixa_renda in [A,B]".
func renderLabel(p *Predicate) string {
	var b strings.Builder
	b.WriteString(p.Field)
	b.WriteByte(' ')
	b.WriteString(string(p.Op))

	switch p.Op {
	case OpExists, OpNotExists, OpTruthy:
		return b.String()
	}

	b.WriteByte(' ')
	b.WriteString(renderValue(p.Value))
	return b.String()
}

func renderValue(v any) string {
	if list, ok := toList(v); ok {
		parts := make([]string, 0, len(list))
		for _, item := range list {
			parts = append(parts, renderValue(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	if s, ok := toText(v); ok {
		return s
	}
	if v == nil {
		return "null"
	}
	return strconv.Quote(fmt.Sprintf("%v", v))
}
