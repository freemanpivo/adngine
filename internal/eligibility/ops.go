package eligibility

import "strings"

// evalFunc recebe found para distinguir campo ausente de campo com valor nulo.
// Salvo exists/not_exists, campo ausente reprova o predicado (RF-10).
type evalFunc func(actual any, found bool, p *Predicate) bool

var operators = map[Op]evalFunc{
	OpEq:        requireFound(func(actual any, p *Predicate) bool { return equal(actual, p.Value) }),
	OpNe:        requireFound(func(actual any, p *Predicate) bool { return !equal(actual, p.Value) }),
	OpGt:        ordered(func(cmp int) bool { return cmp > 0 }),
	OpGte:       ordered(func(cmp int) bool { return cmp >= 0 }),
	OpLt:        ordered(func(cmp int) bool { return cmp < 0 }),
	OpLte:       ordered(func(cmp int) bool { return cmp <= 0 }),
	OpIn:        requireFound(func(actual any, p *Predicate) bool { return inList(actual, p.Value) }),
	OpNotIn:     requireFound(func(actual any, p *Predicate) bool { return !inList(actual, p.Value) }),
	OpContains:  requireFound(contains),
	OpTruthy:    requireFound(func(actual any, _ *Predicate) bool { return truthy(actual) }),
	OpRegex:     requireFound(matchesRegex),
	OpExists:    func(_ any, found bool, _ *Predicate) bool { return found },
	OpNotExists: func(_ any, found bool, _ *Predicate) bool { return !found },
}

// listOperators exigem uma lista em value; a validacao estatica usa isso.
var listOperators = map[Op]bool{OpIn: true, OpNotIn: true}

func knownOp(op Op) bool {
	_, ok := operators[op]
	return ok
}

func requireFound(fn func(actual any, p *Predicate) bool) evalFunc {
	return func(actual any, found bool, p *Predicate) bool {
		if !found {
			return false
		}
		return fn(actual, p)
	}
}

func ordered(accept func(cmp int) bool) evalFunc {
	return requireFound(func(actual any, p *Predicate) bool {
		cmp, ok := compare(actual, p.Value)
		return ok && accept(cmp)
	})
}

func inList(actual, value any) bool {
	list, ok := toList(value)
	if !ok {
		return false
	}
	for _, item := range list {
		if equal(actual, item) {
			return true
		}
	}
	return false
}

// contains atende os dois sentidos naturais: lista que contem o valor e string
// que contem o trecho.
func contains(actual any, p *Predicate) bool {
	if list, ok := toList(actual); ok {
		for _, item := range list {
			if equal(item, p.Value) {
				return true
			}
		}
		return false
	}

	haystack, hok := toText(actual)
	needle, nok := toText(p.Value)
	return hok && nok && strings.Contains(haystack, needle)
}

func matchesRegex(actual any, p *Predicate) bool {
	if p.regex == nil {
		return false
	}
	text, ok := toText(actual)
	return ok && p.regex.MatchString(text)
}
