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
	Detail   string
}

func (e *CaptureError) Error() string { return fmt.Sprintf("erro de captura: %s", e.Detail) }

func captureResponse(session template.Session, declarations []parser.Capture, response Response) *CaptureError {
	batch := make([]template.Capture, 0, len(declarations))
	for _, declaration := range declarations {
		var value any
		var found bool
		switch declaration.Kind {
		case parser.JSONPathCapture:
			var err error
			value, found, err = declaration.Path.Select(response.Body)
			if err != nil {
				return &CaptureError{Position: declaration.QueryPosition, Detail: "resposta não contém JSON válido"}
			}
		case parser.HeaderCapture:
			value, found = query.Header(response.Headers, declaration.Header)
		default:
			return &CaptureError{Position: declaration.QueryPosition, Detail: "variante de captura não suportada"}
		}
		if !found {
			return &CaptureError{Position: declaration.QueryPosition, Detail: "consulta não encontrou valor"}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return &CaptureError{Position: declaration.QueryPosition, Detail: "não foi possível serializar valor da captura"}
		}
		session.ProtectCapture(declaration.Name, raw)
		batch = append(batch, template.Capture{Name: declaration.Name, Value: raw})
	}
	if err := session.Publish(batch); err != nil {
		pos := declarations[0].Position
		var conflict *template.ConflictError
		if errors.As(err, &conflict) && conflict.Index >= 0 && conflict.Index < len(declarations) {
			pos = declarations[conflict.Index].Position
		}
		var invalid *template.CaptureValueError
		if errors.As(err, &invalid) && invalid.Index >= 0 && invalid.Index < len(declarations) {
			pos = declarations[invalid.Index].QueryPosition
		}
		return &CaptureError{Position: pos, Detail: "não foi possível publicar captura; verifique os valores e nomes inválidos, repetidos ou em conflito ancestral/descendente"}
	}
	return nil
}
