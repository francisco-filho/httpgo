package assertion

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// JSONOperator is a predicate on a selected JSON value or its array count.
type JSONOperator string

const (
	JSONEqualOperator    JSONOperator = "=="
	JSONNotEqualOperator JSONOperator = "!="
	JSONMatches          JSONOperator = "matches"
	JSONIsString         JSONOperator = "isString"
	JSONIsInteger        JSONOperator = "isInteger"
	JSONIsBoolean        JSONOperator = "isBoolean"
	JSONIsEmpty          JSONOperator = "isEmpty"
	JSONExists           JSONOperator = "exists"
	JSONNotExists        JSONOperator = "not exists"
	JSONContains         JSONOperator = "contains"
	JSONNotContains      JSONOperator = "not contains"
	JSONGreater          JSONOperator = ">"
	JSONGreaterEqual     JSONOperator = ">="
	JSONLess             JSONOperator = "<"
	JSONLessEqual        JSONOperator = "<="
)

// TypePredicate reports whether the operator checks type or emptiness without an operand.
func (operator JSONOperator) TypePredicate() bool {
	switch operator {
	case JSONIsString, JSONIsInteger, JSONIsBoolean, JSONIsEmpty:
		return true
	default:
		return false
	}
}

// JSONEvaluation contains the compared value, never collections for count,
// membership or absence predicates.
// Detail is controlled type information, never response content.
type JSONEvaluation struct {
	Matched bool
	Actual  any
	Detail  string
}

// ValidateJSONExpected checks applicability independently of the actual response.
// It is shared by static parsing and late expected-value preparation.
func ValidateJSONExpected(operator JSONOperator, count bool, expected any) error {
	if count && operator == JSONNotEqualOperator {
		return fmt.Errorf("count não aceita !=")
	}
	if count && !ValidCount(expected) {
		return fmt.Errorf("count exige quantidade inteira não negativa")
	}
	switch operator {
	case JSONContains, JSONNotContains:
		switch expected.(type) {
		case nil, string, json.Number, bool:
		default:
			return fmt.Errorf("contains e not contains exigem literal JSON escalar")
		}
	case JSONGreater, JSONGreaterEqual, JSONLess, JSONLessEqual:
		if _, ok := expected.(json.Number); !ok {
			return fmt.Errorf("comparação ordenada exige número JSON esperado")
		}
	case JSONEqualOperator, JSONNotEqualOperator, JSONExists, JSONNotExists, JSONMatches:
	default:
		return fmt.Errorf("predicado JSON não suportado")
	}
	return nil
}

// EvaluateJSON decides assertions; query only supplies value and presence.
func EvaluateJSON(operator JSONOperator, count bool, expected, actual any, present bool) (JSONEvaluation, error) {
	if count && operator == JSONNotEqualOperator {
		return JSONEvaluation{}, fmt.Errorf("count não aceita !=")
	}
	if operator.TypePredicate() {
		if count {
			return JSONEvaluation{}, fmt.Errorf("count não aceita predicado de tipo ou vazio")
		}
		return evaluateType(operator, actual, present), nil
	}
	result := JSONEvaluation{Actual: actual}
	if operator == JSONContains || operator == JSONNotContains || operator == JSONNotExists {
		if count {
			return JSONEvaluation{}, fmt.Errorf("count não aceita predicado de inclusão ou ausência")
		}
		if operator == JSONNotExists {
			return JSONEvaluation{Matched: !present}, nil
		}
		return evaluateMembership(operator, expected, actual, present)
	}
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
	case JSONNotEqualOperator:
		result.Matched = present && !JSONEqual(expected, actual)
	case JSONGreater, JSONGreaterEqual, JSONLess, JSONLessEqual:
		b, ok := expected.(json.Number)
		if !ok {
			return JSONEvaluation{}, fmt.Errorf("comparação JSON exige esperado numérico")
		}
		if !present {
			return JSONEvaluation{}, nil
		}
		a, ok := actual.(json.Number)
		if !ok {
			return JSONEvaluation{Detail: "comparação ordenada exige número; recebido " + JSONType(actual)}, nil
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

// evaluateType keeps only categorical information, never the selected operand.
func evaluateType(operator JSONOperator, actual any, present bool) JSONEvaluation {
	result := JSONEvaluation{}
	if !present {
		result.Detail = "valor ausente"
		return result
	}
	result.Detail = "tipo " + JSONType(actual)
	switch operator {
	case JSONIsString:
		_, result.Matched = actual.(string)
	case JSONIsBoolean:
		_, result.Matched = actual.(bool)
	case JSONIsInteger:
		if number, ok := actual.(json.Number); ok {
			// Decode has already validated JSON grammar. Keep the original notation
			// and precision; mathematical integrality is a different predicate.
			result.Matched = !strings.ContainsAny(string(number), ".eE")
			if !result.Matched {
				result.Detail += "; notação não inteira"
			}
		}
	case JSONIsEmpty:
		switch value := actual.(type) {
		case string:
			result.Matched = len(value) == 0
		case []any:
			result.Matched = len(value) == 0
		case map[string]any:
			result.Matched = len(value) == 0
		default:
			result.Detail += "; isEmpty exige string, array ou objeto"
		}
	}
	if !result.Matched {
		result.Detail += "; condição falsa"
	}
	return result
}

// evaluateMembership separates absence and applicability from the condition:
// negating an inapplicable predicate must not turn it into a success.
func evaluateMembership(operator JSONOperator, expected, actual any, present bool) (JSONEvaluation, error) {
	switch expected.(type) {
	case nil, string, json.Number, bool:
	default:
		return JSONEvaluation{}, fmt.Errorf("pertencimento JSON exige esperado escalar")
	}
	result := JSONEvaluation{}
	if !present {
		result.Matched = operator == JSONNotContains
		return result, nil
	}
	contains := false
	switch value := actual.(type) {
	case string:
		text, ok := expected.(string)
		if !ok {
			result.Detail = "busca em string exige esperado string"
			return result, nil
		}
		contains = strings.Contains(value, text)
	case []any:
		for _, element := range value {
			if JSONEqual(expected, element) {
				contains = true
				break
			}
		}
	default:
		result.Detail = "pertencimento exige string ou array; recebido " + JSONType(actual)
		return result, nil
	}
	result.Matched = contains
	if operator == JSONNotContains {
		result.Matched = !contains
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
