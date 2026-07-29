package eligibility

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Numero vindo do DynamoDB chega como string e numero vindo de JSON chega como
// float64. As comparacoes normalizam antes de decidir, para que o mesmo YAML
// funcione com as duas origens (RF-10).
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		trimmed := strings.TrimSpace(n)
		if !looksNumeric(trimmed) {
			return 0, false
		}
		f, err := strconv.ParseFloat(trimmed, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// looksNumeric evita chamar ParseFloat com texto comum: o caminho de erro do
// strconv aloca, e comparar string com string e o caso mais frequente do motor.
func looksNumeric(s string) bool {
	if s == "" {
		return false
	}
	digits := false
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			digits = true
		case c == '+' || c == '-' || c == '.' || c == 'e' || c == 'E':
		default:
			return false
		}
	}
	return digits
}

var (
	trueLiterals  = map[string]bool{"true": true, "t": true, "yes": true, "y": true, "sim": true, "1": true}
	falseLiterals = map[string]bool{"false": true, "f": true, "no": true, "n": true, "nao": true, "0": true}
)

// toBool so aceita booleano e as representacoes textuais canonicas. Strings
// arbitrarias nao viram booleano aqui: quem decide isso e truthy.
func toBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		lower := strings.ToLower(strings.TrimSpace(b))
		if trueLiterals[lower] {
			return true, true
		}
		if falseLiterals[lower] {
			return false, true
		}
		return false, false
	default:
		return false, false
	}
}

// truthy e mais permissivo que toBool: qualquer string nao vazia que nao seja um
// literal falso conta como verdadeira, o que cobre flags como "S" do DynamoDB.
func truthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		lower := strings.ToLower(strings.TrimSpace(t))
		return lower != "" && !falseLiterals[lower]
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	default:
		if f, ok := toFloat(v); ok {
			return f != 0
		}
		return true
	}
}

func toText(v any) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case fmt.Stringer:
		return s.String(), true
	case nil:
		return "", false
	default:
		if f, ok := toFloat(v); ok {
			return strconv.FormatFloat(f, 'f', -1, 64), true
		}
		if b, ok := v.(bool); ok {
			return strconv.FormatBool(b), true
		}
		return "", false
	}
}

// equal compara na ordem numero, booleano, texto. Tipos incomparaveis devolvem
// false em vez de panic.
func equal(a, b any) bool {
	if af, ok := toFloat(a); ok {
		if bf, ok := toFloat(b); ok {
			return af == bf
		}
	}

	_, aIsBool := a.(bool)
	_, bIsBool := b.(bool)
	if aIsBool || bIsBool {
		ab, aok := toBool(a)
		bb, bok := toBool(b)
		return aok && bok && ab == bb
	}

	as, aok := toText(a)
	bs, bok := toText(b)
	return aok && bok && as == bs
}

// compare devolve -1, 0 ou 1 e false quando os valores nao sao ordenaveis entre
// si.
func compare(a, b any) (int, bool) {
	if af, ok := toFloat(a); ok {
		if bf, ok := toFloat(b); ok {
			switch {
			case af < bf:
				return -1, true
			case af > bf:
				return 1, true
			default:
				return 0, true
			}
		}
	}

	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		return strings.Compare(as, bs), true
	}
	return 0, false
}
