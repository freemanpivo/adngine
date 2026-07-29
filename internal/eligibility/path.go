package eligibility

import (
	"strconv"
	"strings"
)

// Lookup resolve um caminho como "a.b[0].c" sobre a bag. Devolve false quando
// qualquer segmento nao existe, em vez de erro: campo ausente e um resultado
// legitimo para o motor (RF-10).
//
// O caminho e percorrido sem fatiar em slice porque Lookup roda uma vez por
// predicado avaliado, no hot path da selecao.
func Lookup(bag map[string]any, path string) (any, bool) {
	if path == "" || bag == nil {
		return nil, false
	}

	var current any = bag
	rest := path
	for rest != "" {
		segment, remainder, _ := strings.Cut(rest, ".")
		rest = remainder

		resolved, ok := resolveSegment(current, segment)
		if !ok {
			return nil, false
		}
		current = resolved
	}
	return current, true
}

func resolveSegment(current any, segment string) (any, bool) {
	name, indexes, _ := strings.Cut(segment, "[")
	if name == "" && indexes == "" {
		return nil, false
	}

	if name != "" {
		m, ok := toStringMap(current)
		if !ok {
			return nil, false
		}
		current, ok = m[name]
		if !ok {
			return nil, false
		}
	}

	for indexes != "" {
		digits, remainder, closed := strings.Cut(indexes, "]")
		if !closed {
			return nil, false
		}
		idx, err := strconv.Atoi(digits)
		if err != nil {
			return nil, false
		}

		item, ok := elementAt(current, idx)
		if !ok {
			return nil, false
		}
		current = item

		if remainder != "" {
			if remainder[0] != '[' {
				return nil, false
			}
			remainder = remainder[1:]
		}
		indexes = remainder
	}
	return current, true
}

func elementAt(v any, idx int) (any, bool) {
	if idx < 0 {
		return nil, false
	}
	switch list := v.(type) {
	case []any:
		if idx >= len(list) {
			return nil, false
		}
		return list[idx], true
	case []string:
		if idx >= len(list) {
			return nil, false
		}
		return list[idx], true
	default:
		return nil, false
	}
}

func toList(v any) ([]any, bool) {
	switch l := v.(type) {
	case []any:
		return l, true
	case []string:
		out := make([]any, len(l))
		for i, item := range l {
			out[i] = item
		}
		return out, true
	default:
		return nil, false
	}
}
