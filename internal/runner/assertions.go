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
			if present && assertion.JSONEqual(declaration.Expected, actual) {
				continue
			}
			expectedText, actualText := "[oculto]", "[oculto]"
			if !conceal {
				expectedJSON, err := json.Marshal(declaration.Expected)
				if err != nil {
					return fail("não foi possível representar o esperado JSON")
				}
				actualJSON, err := json.Marshal(actual)
				if err != nil {
					return fail("não foi possível representar o valor JSON selecionado")
				}
				expectedText, actualText = session.Sanitize(string(expectedJSON)), session.Sanitize(string(actualJSON))
			}
			current.JSONFailure = &JSONFailure{Position: pos, Query: session.Sanitize(declaration.Query), Expected: expectedText, Actual: actualText, Present: present}
			current.Outcome = JSONMismatch
			return nil
		default:
			current.Outcome = JSONFailed
			return &JSONError{Position: entry.Position, Detail: "variante de assertion não suportada"}
		}
	}
	return nil
}
