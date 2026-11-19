package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"capimgrosso.com/httpgo/internal/template"
)

// terminalDiagnostic copies controlled messages only. Operational error causes
// stay in Result.Err and are never passed to the synchronous entry callback.
func terminalDiagnostic(result Result, session template.Session, options Options) *EntryDiagnostic {
	if result.Err == nil {
		return nil
	}
	d := &EntryDiagnostic{Position: result.ErrorPosition, Phase: "transporte", Detail: "falha de transporte; verifique conexão, TLS e redirecionamentos"}
	var safe interface{ SafeDiagnostic() string }
	if errors.As(result.Err, &safe) {
		d.Detail = safe.SafeDiagnostic()
	}
	var render *RenderError
	var file *FileError
	var capture *CaptureError
	var json *JSONError
	var header *HeaderError
	var body *BodyError
	var regex *RegexError
	switch {
	case errors.As(result.Err, &render):
		d.Phase, d.Detail = "renderização", render.Error()
	case errors.As(result.Err, &file):
		d.Phase, d.Detail = "renderização", file.Error()
	case errors.As(result.Err, &capture):
		d.Phase, d.Detail, d.Query, d.Count = "captures", capture.Detail, capture.Query, capture.Count
	case errors.As(result.Err, &json):
		d.Phase, d.Detail, d.Query = "assertion", json.Detail, json.Query
	case errors.As(result.Err, &header):
		d.Phase, d.Detail = "assertion", header.Detail
	case errors.As(result.Err, &body):
		d.Phase, d.Detail = "assertion", body.Detail
	case errors.As(result.Err, &regex):
		d.Phase, d.Detail = "assertion", regex.Detail
	case errors.Is(result.Err, context.Canceled):
		d.Detail = "requisição cancelada"
	case errors.Is(result.Err, context.DeadlineExceeded):
		timeout := options.Timeout
		if timeout == 0 {
			timeout = 10 * time.Second
		}
		d.Detail = fmt.Sprintf("timeout da requisição (limite total de %g segundos)", timeout.Seconds())
	}
	d.Position.File = session.Sanitize(d.Position.File)
	d.Detail, d.Query = session.Sanitize(d.Detail), session.Sanitize(d.Query)
	return d
}
