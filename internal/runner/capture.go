package runner

import (
	"encoding/json"
	"errors"
	"fmt"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

type CaptureError struct {
	Position parser.Position
	Detail   string
}

func (e *CaptureError) Error() string { return fmt.Sprintf("erro de captura: %s", e.Detail) }

func captureResponse(session template.Session, declarations []parser.Capture, body []byte) *CaptureError {
	batch := make([]template.Capture, 0, len(declarations))
	for _, declaration := range declarations {
		value, found, err := declaration.Path.Select(body)
		if err != nil {
			return &CaptureError{Position: declaration.QueryPosition, Detail: "resposta não contém JSON válido"}
		}
		if !found {
			return &CaptureError{Position: declaration.QueryPosition, Detail: "consulta não encontrou valor"}
		}
		text, ok := value.(string)
		if !ok {
			return &CaptureError{Position: declaration.QueryPosition, Detail: "consulta exige resultado string"}
		}
		raw, err := json.Marshal(text)
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
		return &CaptureError{Position: pos, Detail: "nome de captura inválido, repetido ou em conflito ancestral/descendente"}
	}
	return nil
}
