package assertion

import (
	"encoding/json"
	"math/big"
	"strings"
)

// JSONEqual compares decoded JSON values structurally, without coercion.
// Numbers must be json.Number, as produced by a decoder with UseNumber.
func JSONEqual(expected, actual any) bool {
	switch expected := expected.(type) {
	case nil:
		return actual == nil
	case string:
		value, ok := actual.(string)
		return ok && expected == value
	case bool:
		value, ok := actual.(bool)
		return ok && expected == value
	case json.Number:
		value, ok := actual.(json.Number)
		return ok && equalNumber(expected, value)
	case []any:
		value, ok := actual.([]any)
		if !ok || len(expected) != len(value) {
			return false
		}
		for i := range expected {
			if !JSONEqual(expected[i], value[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		value, ok := actual.(map[string]any)
		if !ok || len(expected) != len(value) {
			return false
		}
		for key, item := range expected {
			other, present := value[key]
			if !present || !JSONEqual(item, other) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func equalNumber(a, b json.Number) bool {
	ac, ae := decimal(a.String())
	bc, be := decimal(b.String())
	return ac == bc && ae.Cmp(be) == 0
}

// decimal normalizes valid JSON numbers into signed significant digits and a
// decimal exponent. It never expands powers of ten: memory follows input length,
// even for exponents larger than a machine integer.
func decimal(source string) (string, *big.Int) {
	mantissa, exponent, _ := strings.Cut(strings.ToLower(source), "e")
	power := new(big.Int)
	if exponent != "" {
		power.SetString(exponent, 10)
	}
	negative := strings.HasPrefix(mantissa, "-")
	mantissa = strings.TrimPrefix(mantissa, "-")
	whole, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return "0", new(big.Int)
	}
	trimmed := strings.TrimRight(digits, "0")
	adjustment := len(digits) - len(trimmed) - len(fraction)
	power.Add(power, big.NewInt(int64(adjustment)))
	if negative {
		trimmed = "-" + trimmed
	}
	return trimmed, power
}
