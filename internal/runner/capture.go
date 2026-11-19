package runner

import (
	"encoding/json"
	"errors"
	"fmt"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

type CaptureError struct {
	Position parser.Position
	Query    string
	Count    bool
	Detail   string
}

func (e *CaptureError) Error() string {
	if e.Count {
		return fmt.Sprintf("erro de captura: consulta %q count: %s", e.Query, e.Detail)
	}
	return fmt.Sprintf("erro de captura: %s", e.Detail)
}

func captureFailure(session template.Session, declaration parser.Capture, pos parser.Position, detail string) *CaptureError {
	pos.File = session.Sanitize(pos.File)
	return &CaptureError{Position: pos, Query: session.Sanitize(declaration.Path.RedactLiterals(declaration.Query)), Count: declaration.Count, Detail: session.Sanitize(detail)}
}

func extractCaptures(session template.Session, declarations []parser.Capture, response Response, extracted func(int)) ([]template.Capture, *CaptureError) {
	batch := make([]template.Capture, 0, len(declarations))
	var text string
	textDecoded := false
	for index, declaration := range declarations {
		fail := func(detail string) *CaptureError {
			return captureFailure(session, declaration, declaration.QueryPosition, detail)
		}
		var value any
		var found bool
		switch declaration.Kind {
		case parser.JSONPathCapture:
			bound, secret, err := bindQuery(session, declaration.Path)
			if secret {
				session.MarkSecret(declaration.Name)
			}
			if err != nil {
				return nil, fail("não foi possível resolver operando JSONPath: " + err.Error())
			}
			value, found, err = bound.Select(response.Body)
			if err != nil {
				return nil, fail(err.Error())
			}
		case parser.HeaderCapture:
			value, found = query.Header(response.Headers, declaration.Header)
		case parser.RegexCapture:
			if !textDecoded {
				var err error
				text, err = query.BodyText(response.Body, response.Headers)
				if err != nil {
					return nil, fail(err.Error())
				}
				textDecoded = true
			}
			value, found = text, true
		default:
			return nil, fail("variante de captura não suportada")
		}
		if !found {
			return nil, fail("consulta não encontrou valor")
		}
		if declaration.Count {
			quantity, ok := query.CountArray(value)
			if !ok {
				return nil, fail("count exige array")
			}
			value = quantity
		}
		if declaration.Kind == parser.RegexCapture || declaration.Compiled != nil {
			selected, err := query.Regex(declaration.Compiled, value.(string))
			if err != nil {
				return nil, captureFailure(session, declaration, declaration.PatternPosition, err.Error())
			}
			value = selected
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fail("não foi possível serializar valor da captura")
		}
		session.ProtectCapture(declaration.Name, raw)
		if extracted != nil {
			extracted(index)
		}
		batch = append(batch, template.Capture{Name: declaration.Name, Value: raw})
	}
	return batch, nil
}

func captureBatchFailure(session template.Session, declarations []parser.Capture, err error) *CaptureError {
	declaration := declarations[0]
	pos := declaration.Position
	var conflict *template.ConflictError
	if errors.As(err, &conflict) && conflict.Index >= 0 && conflict.Index < len(declarations) {
		declaration = declarations[conflict.Index]
		pos = declaration.Position
	}
	var invalid *template.CaptureValueError
	if errors.As(err, &invalid) && invalid.Index >= 0 && invalid.Index < len(declarations) {
		declaration = declarations[invalid.Index]
		pos = declaration.QueryPosition
	}
	return captureFailure(session, declaration, pos, "não foi possível publicar captura; verifique os valores e nomes inválidos, repetidos ou em conflito ancestral/descendente")
}
