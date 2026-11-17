package query

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
		return ok && CompareNumber(expected, value) == 0
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

// CompareNumber compares normalized decimals without expanding powers of ten.
// The exponent can exceed machine integers; padding is bounded by input digits.
func CompareNumber(a, b json.Number) int {
	ac, ae := decimal(a.String())
	bc, be := decimal(b.String())
	if ac == "0" || bc == "0" {
		switch {
		case ac == bc:
			return 0
		case ac == "0":
			if strings.HasPrefix(bc, "-") {
				return 1
			}
			return -1
		case strings.HasPrefix(ac, "-"):
			return -1
		default:
			return 1
		}
	}
	an, bn := strings.HasPrefix(ac, "-"), strings.HasPrefix(bc, "-")
	if an != bn {
		if an {
			return -1
		}
		return 1
	}
	ac, bc = strings.TrimPrefix(ac, "-"), strings.TrimPrefix(bc, "-")
	ae.Add(ae, big.NewInt(int64(len(ac))))
	be.Add(be, big.NewInt(int64(len(bc))))
	order := ae.Cmp(be)
	if order == 0 {
		length := max(len(ac), len(bc))
		order = strings.Compare(ac+strings.Repeat("0", length-len(ac)), bc+strings.Repeat("0", length-len(bc)))
	}
	if an {
		return -order
	}
	return order
}

// ValidCount accepts numeric values that are integral and nonnegative, without
// imposing an integer-machine limit (including exact decimal/exponent forms).
func ValidCount(value any) bool {
	number, ok := value.(json.Number)
	if !ok {
		return false
	}
	digits, power := decimal(number.String())
	return !strings.HasPrefix(digits, "-") && power.Sign() >= 0
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
