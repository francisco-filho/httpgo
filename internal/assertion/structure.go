package assertion

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// JSONOperator is a predicate on a selected JSON value or its array count.
type JSONOperator string

const (
	JSONEqualOperator JSONOperator = "=="
	JSONExists        JSONOperator = "exists"
	JSONGreater       JSONOperator = ">"
	JSONGreaterEqual  JSONOperator = ">="
	JSONLess          JSONOperator = "<"
	JSONLessEqual     JSONOperator = "<="
)

// JSONEvaluation contains the compared value, not the selected array for count.
// Detail is controlled type information, never response content.
type JSONEvaluation struct {
	Matched bool
	Actual  any
	Detail  string
}

// EvaluateJSON decides assertions; query only supplies value and presence.
func EvaluateJSON(operator JSONOperator, count bool, expected, actual any, present bool) (JSONEvaluation, error) {
	result := JSONEvaluation{Actual: actual}
	if count {
		if !present {
			return JSONEvaluation{}, nil
		}
		array, ok := actual.([]any)
		if !ok {
			return JSONEvaluation{Detail: "count exige array; recebido " + JSONType(actual)}, nil
		}
		actual = json.Number(strconv.Itoa(len(array)))
		result.Actual = actual
	}
	switch operator {
	case JSONExists:
		if count {
			return JSONEvaluation{}, fmt.Errorf("count não aceita exists")
		}
		result.Matched = present
	case JSONEqualOperator:
		result.Matched = present && JSONEqual(expected, actual)
	case JSONGreater, JSONGreaterEqual, JSONLess, JSONLessEqual:
		if !count {
			return JSONEvaluation{}, fmt.Errorf("comparador JSON direto não suportado")
		}
		a, aok := actual.(json.Number)
		b, bok := expected.(json.Number)
		if !aok || !bok {
			return JSONEvaluation{}, fmt.Errorf("comparação count exige números")
		}
		order := compareNumber(a, b)
		switch operator {
		case JSONGreater:
			result.Matched = order > 0
		case JSONGreaterEqual:
			result.Matched = order >= 0
		case JSONLess:
			result.Matched = order < 0
		case JSONLessEqual:
			result.Matched = order <= 0
		default:
			return JSONEvaluation{}, fmt.Errorf("comparador JSON não suportado")
		}
	default:
		return JSONEvaluation{}, fmt.Errorf("predicado JSON não suportado")
	}
	return result, nil
}

// JSONType names JSON types without exposing any operand content.
func JSONType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case json.Number:
		return "número"
	case bool:
		return "booleano"
	case []any:
		return "array"
	case map[string]any:
		return "objeto"
	default:
		return "tipo não suportado"
	}
}
