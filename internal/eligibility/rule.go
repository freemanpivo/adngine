// Package eligibility avalia regras declarativas contra uma bag de atributos de
// schema desconhecido. O pacote e puro: nao faz I/O e nao conhece o inventario.
package eligibility

import (
	"errors"
	"fmt"
	"regexp"
)

type Op string

const (
	OpEq        Op = "eq"
	OpNe        Op = "ne"
	OpGt        Op = "gt"
	OpGte       Op = "gte"
	OpLt        Op = "lt"
	OpLte       Op = "lte"
	OpIn        Op = "in"
	OpNotIn     Op = "not_in"
	OpContains  Op = "contains"
	OpExists    Op = "exists"
	OpNotExists Op = "not_exists"
	OpTruthy    Op = "truthy"
	OpRegex     Op = "regex"
)

type Kind uint8

const (
	KindPredicate Kind = iota
	KindAll
	KindAny
	KindNot
)

// Rule e um no da arvore: ou um predicado folha, ou um combinador com filhos.
// KindNot tem exatamente um filho.
type Rule struct {
	Kind      Kind
	Children  []Rule
	Predicate *Predicate
}

type Predicate struct {
	Field    string
	Op       Op
	Value    any
	Weight   float64
	Required bool
	LogValue bool

	// Preenchidos por Compile (RF-35) para que o trace nao formate string no
	// hot path.
	ID    string
	Label string
	regex *regexp.Regexp
}

const (
	defaultWeight   = 1.0
	defaultRequired = true
	defaultLogValue = true
)

var combinators = map[string]Kind{
	"all": KindAll,
	"any": KindAny,
	"not": KindNot,
}

// ParseRule converte o mapa cru vindo do YAML na arvore tipada. Um no e
// combinador quando declara all/any/not e predicado caso contrario.
func ParseRule(raw map[string]any) (Rule, error) {
	return parseNode(raw, "rule")
}

func parseNode(raw map[string]any, path string) (Rule, error) {
	if len(raw) == 0 {
		return Rule{}, fmt.Errorf("%s: no vazio", path)
	}

	var found []string
	for key := range combinators {
		if _, ok := raw[key]; ok {
			found = append(found, key)
		}
	}
	switch len(found) {
	case 0:
		return parsePredicate(raw, path)
	case 1:
		return parseCombinator(found[0], raw, path)
	default:
		return Rule{}, fmt.Errorf("%s: no declara mais de um combinador", path)
	}
}

func parseCombinator(key string, raw map[string]any, path string) (Rule, error) {
	if len(raw) > 1 {
		return Rule{}, fmt.Errorf("%s: combinador %q nao aceita outros campos no mesmo no", path, key)
	}

	kind := combinators[key]
	nested := fmt.Sprintf("%s.%s", path, key)

	if kind == KindNot {
		child, err := parseChild(raw[key], nested)
		if err != nil {
			return Rule{}, err
		}
		return Rule{Kind: KindNot, Children: []Rule{child}}, nil
	}

	list, ok := raw[key].([]any)
	if !ok {
		return Rule{}, fmt.Errorf("%s: %q espera uma lista de regras", path, key)
	}
	if len(list) == 0 {
		return Rule{}, fmt.Errorf("%s: %q espera ao menos uma regra", path, key)
	}

	var errs []error
	children := make([]Rule, 0, len(list))
	for i, item := range list {
		child, err := parseChild(item, fmt.Sprintf("%s[%d]", nested, i))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		children = append(children, child)
	}
	if err := errors.Join(errs...); err != nil {
		return Rule{}, err
	}
	return Rule{Kind: kind, Children: children}, nil
}

func parseChild(raw any, path string) (Rule, error) {
	child, ok := toStringMap(raw)
	if !ok {
		return Rule{}, fmt.Errorf("%s: esperava um mapa, recebeu %T", path, raw)
	}
	return parseNode(child, path)
}

func parsePredicate(raw map[string]any, path string) (Rule, error) {
	p := Predicate{
		Weight:   defaultWeight,
		Required: defaultRequired,
		LogValue: defaultLogValue,
	}

	var errs []error
	for key, value := range raw {
		var err error
		switch key {
		case "field":
			p.Field, err = asString(value)
		case "op":
			var op string
			if op, err = asString(value); err == nil {
				p.Op = Op(op)
			}
		case "value":
			p.Value = value
		case "weight":
			p.Weight, err = asFloat(value)
		case "required":
			p.Required, err = asBool(value)
		case "log_value":
			p.LogValue, err = asBool(value)
		default:
			err = errors.New("campo desconhecido em predicado")
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s.%s: %w", path, key, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return Rule{}, err
	}
	return Rule{Kind: KindPredicate, Predicate: &p}, nil
}

// toStringMap aceita map[any]any porque nem todo decoder de YAML entrega chaves
// ja tipadas como string.
func toStringMap(raw any) (map[string]any, bool) {
	switch m := raw.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, v := range m {
			key, ok := k.(string)
			if !ok {
				return nil, false
			}
			out[key] = v
		}
		return out, true
	default:
		return nil, false
	}
}

func asString(v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("esperava string, recebeu %T", v)
	}
	return s, nil
}

func asBool(v any) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("esperava booleano, recebeu %T", v)
	}
	return b, nil
}

func asFloat(v any) (float64, error) {
	f, ok := toFloat(v)
	if !ok {
		return 0, fmt.Errorf("esperava numero, recebeu %T", v)
	}
	return f, nil
}
