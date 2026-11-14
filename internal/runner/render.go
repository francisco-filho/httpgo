package runner

import (
	"errors"
	"slices"
	"strings"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

func renderFailure(pos parser.Position, err error) *RenderError {
	detail := "não foi possível resolver o texto"
	var located *template.Error
	if errors.As(err, &located) {
		pos.Column += located.Offset
		detail = located.Detail
	}
	return &RenderError{Position: pos, Detail: detail, cause: err}
}

func (r *Runner) render(entry parser.Entry) (Request, string, *RenderError) {
	pos := entry.URLPosition
	if pos.Line == 0 {
		pos = entry.Position
	}
	url, err := r.renderer.Render(entry.URL)
	if err != nil {
		return Request{}, "", renderFailure(pos, err)
	}
	// Static URLs were fully checked by the parser; stricter URI byte checks
	// prevent implicit percent-encoding of newly inserted values.
	if strings.Contains(entry.URL, "{{") {
		if err := parser.ValidateURL(url.Value); err != nil {
			return Request{}, "", &RenderError{Position: pos, Detail: err.Error(), cause: err}
		}
	}
	headers := slices.Clone(entry.Headers)
	for i := range headers {
		pos = entry.Position
		if i < len(entry.HeaderPositions) {
			pos = entry.HeaderPositions[i]
		}
		text, err := r.renderer.Render(headers[i].Value)
		if err != nil {
			return Request{}, "", renderFailure(pos, err)
		}
		if err := parser.ValidateHeader(headers[i].Name, text.Value); err != nil {
			return Request{}, "", &RenderError{Position: pos, Detail: err.Error(), cause: err}
		}
		headers[i].Value = text.Value
	}
	return Request{Method: entry.Method, URL: url.Value, Headers: headers, Body: slices.Clone(entry.Body)}, url.Display, nil
}
