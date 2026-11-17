package runner

import (
	"encoding/json"
	"fmt"

	"capimgrosso.com/httpgo/internal/assertion"
	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

// projectionFailure summarizes the reduced value before any collection can be
// serialized for presentation. Predicate evaluation still receives the real value.
func projectionFailure(session template.Session, declaration parser.JSONAssertion, pos parser.Position, actual any, present bool, evaluation assertion.JSONEvaluation, conceal bool) *JSONFailure {
	failure := &JSONFailure{Position: pos, Query: session.Sanitize(declaration.Query), Present: present, Operator: declaration.Operator, Count: declaration.Count}
	if conceal || protectedJSON(session, actual) || protectedJSON(session, declaration.Expected) || session.Sanitize(declaration.Pattern) != declaration.Pattern {
		failure.Expected, failure.Actual = "[oculto]", "[oculto]"
		return failure
	}
	failure.Detail = session.Sanitize(evaluation.Detail)
	failure.Actual = "ausente"
	if present {
		failure.Actual = "tipo " + assertion.JSONType(actual)
		if array, ok := actual.([]any); ok {
			failure.Actual += fmt.Sprintf("; contagem=%d", len(array))
		}
	}
	switch declaration.Operator {
	case assertion.JSONExists:
		failure.Expected = "presente"
	case assertion.JSONNotExists:
		failure.Expected = "ausente"
	case assertion.JSONMatches:
		encoded, _ := json.Marshal(declaration.Pattern)
		failure.Expected = session.Sanitize(string(encoded))
	default:
		switch declaration.Expected.(type) {
		case []any, map[string]any:
			failure.Expected = "tipo " + assertion.JSONType(declaration.Expected)
		default:
			encoded, _ := json.Marshal(declaration.Expected)
			failure.Expected = session.Sanitize(string(encoded))
		}
	}
	return failure
}

// Inspect leaves without serializing the selected collection. Protected content
// also protects summaries (including lengths), not just literal operands.
func protectedJSON(session template.Session, value any) bool {
	switch value := value.(type) {
	case []any:
		for _, item := range value {
			if protectedJSON(session, item) {
				return true
			}
		}
	case map[string]any:
		for key, item := range value {
			if session.Sanitize(key) != key || protectedJSON(session, item) {
				return true
			}
		}
	case string:
		return session.Sanitize(value) != value
	default:
		encoded, err := json.Marshal(value)
		return err == nil && session.Sanitize(string(encoded)) != string(encoded)
	}
	return false
}
