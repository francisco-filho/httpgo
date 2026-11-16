package runner

import (
	"encoding/json"
	"fmt"

	"capimgrosso.com/httpgo/internal/assertion"
	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// JSONFailure contains only sanitized presentation values, not the document.
// Operands are encoded JSON, so strings, numbers and null remain distinguishable.
type JSONFailure struct {
	Position         parser.Position
	Query            string
	Expected, Actual string
	Present          bool
	Operator         assertion.JSONOperator
	Count            bool
	Detail           string
}

// JSONError is a controlled evaluation error, never a raw decoding cause.
type JSONError struct {
	Position      parser.Position
	Query, Detail string
}

func (e *JSONError) Error() string {
	return fmt.Sprintf("erro de avaliação JSON: consulta %q: %s", e.Query, e.Detail)
}

func checkAssertions(session template.Session, entry parser.Entry, response Response, current *EntryResult) *JSONError {
	var document any
	decoded := false
	conceal := pendingSecret(session, entry)
	for _, item := range entry.Assertions {
		switch {
		case item.Header != nil && item.JSON == nil:
			current.HeaderFailure = checkHeader(session, *item.Header, response.Headers, conceal)
			if current.HeaderFailure != nil {
				current.Outcome = HeaderMismatch
				return nil
			}
		case item.JSON != nil && item.Header == nil:
			declaration := item.JSON
			pos := declaration.Position
			pos.File = session.Sanitize(pos.File)
			fail := func(detail string) *JSONError {
				current.Outcome = JSONFailed
				return &JSONError{Position: pos, Query: session.Sanitize(declaration.Query), Detail: detail}
			}
			if !decoded {
				var err error
				document, err = query.Decode(response.Body)
				if err != nil {
					return fail("resposta não contém um único valor JSON válido")
				}
				decoded = true
			}
			actual, present, err := declaration.Path.SelectValue(document)
			if err != nil {
				return fail(err.Error())
			}
			var evaluation assertion.JSONEvaluation
			if declaration.Operator == assertion.JSONMatches {
				_, isString := actual.(string)
				evaluation = assertion.JSONEvaluation{Actual: actual, Matched: assertion.MatchString(declaration.Compiled, actual, present)}
				if present && !isString {
					evaluation.Detail = "matches exige string; recebido " + assertion.JSONType(actual)
				}
			} else {
				evaluation, err = assertion.EvaluateJSON(declaration.Operator, declaration.Count, declaration.Expected, actual, present)
			}
			if err != nil {
				return fail(err.Error())
			}
			if evaluation.Matched {
				continue
			}
			expectedText, actualText := "[oculto]", "[oculto]"
			if !conceal {
				expectedJSON, err := json.Marshal(declaration.Expected)
				if err != nil {
					return fail("não foi possível representar o esperado JSON")
				}
				actualJSON, err := json.Marshal(evaluation.Actual)
				if err != nil {
					return fail("não foi possível representar o valor JSON selecionado")
				}
				expectedText, actualText = string(expectedJSON), string(actualJSON)
				if declaration.Operator == assertion.JSONExists {
					expectedText, actualText = "presente", "presente"
				} else if declaration.Operator == assertion.JSONMatches {
					patternJSON, _ := json.Marshal(declaration.Pattern)
					actualJSON, _ := json.Marshal(evaluation.Actual)
					expectedText = string(patternJSON)
					if present {
						actualText = string(actualJSON)
					} else {
						actualText = "ausente"
					}
				} else if evaluation.Detail != "" {
					actualText = "tipo " + assertion.JSONType(actual)
				}
				expectedText, actualText = session.Sanitize(expectedText), session.Sanitize(actualText)
			}
			current.JSONFailure = &JSONFailure{Position: pos, Query: session.Sanitize(declaration.Query), Expected: expectedText, Actual: actualText, Present: present, Operator: declaration.Operator, Count: declaration.Count, Detail: session.Sanitize(evaluation.Detail)}
			current.Outcome = JSONMismatch
			return nil
		default:
			current.Outcome = JSONFailed
			return &JSONError{Position: entry.Position, Detail: "variante de assertion não suportada"}
		}
	}
	return nil
}
