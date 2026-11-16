package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

func renderFailure(pos parser.Position, source string, err error) *RenderError {
	detail := "não foi possível resolver o texto"
	var located *template.Error
	if errors.As(err, &located) {
		prefix := source[:min(max(located.Offset, 0), len(source))]
		if lines := strings.Count(prefix, "\n"); lines > 0 {
			pos.Line += lines
			pos.Column = len(prefix) - strings.LastIndexByte(prefix, '\n')
		} else {
			pos.Column += len(prefix)
		}
		detail = located.Detail
	}
	return &RenderError{Position: pos, Detail: detail, cause: err}
}

func renderExternal(session template.Session, source []byte, reference parser.FileBody) ([]byte, *RenderError) {
	// Positions always index the original payload, never the rendered bytes or
	// the resolved operational path. Unknown offsets fall back to its start.
	pos := parser.Position{File: session.Sanitize(reference.Path), Line: 1, Column: 1}
	body, err := session.RenderJSON(source)
	var failure *RenderError
	if err != nil {
		failure = renderFailure(pos, string(source), err)
	} else if !json.Valid(body) {
		failure = &RenderError{Position: pos, Detail: "body JSON renderizado inválido"}
	}
	if failure != nil {
		ref := reference.Position
		ref.File = session.Sanitize(ref.File)
		failure.Reference = &ref
		failure.Detail = session.Sanitize(failure.Detail)
		return nil, failure
	}
	return body, nil
}

func render(session template.Session, entry parser.Entry) (Request, string, *RenderError) {
	pos := entry.URLPosition
	if pos.Line == 0 {
		pos = entry.Position
	}
	url, err := session.Render(entry.URL)
	if err != nil {
		return Request{}, "", renderFailure(pos, entry.URL, err)
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
		text, err := session.Render(headers[i].Value)
		if err != nil {
			return Request{}, "", renderFailure(pos, headers[i].Value, err)
		}
		if err := parser.ValidateHeader(headers[i].Name, text.Value); err != nil {
			return Request{}, "", &RenderError{Position: pos, Detail: err.Error(), cause: err}
		}
		headers[i].Value = text.Value
	}
	body := bytes.Clone(entry.Body)
	if entry.Body != nil {
		pos = entry.BodyPosition
		if pos.Line == 0 {
			pos = entry.Position
		}
		body, err = session.RenderJSON(body)
		if err != nil {
			return Request{}, "", renderFailure(pos, string(entry.Body), err)
		}
		if !json.Valid(body) {
			return Request{}, "", &RenderError{Position: pos, Detail: "body JSON renderizado inválido"}
		}
	}
	retainBody := len(entry.Captures) > 0
	for _, declaration := range entry.Assertions {
		retainBody = retainBody || declaration.JSON != nil || declaration.Body != nil
	}
	return Request{Method: entry.Method, URL: url.Value, Headers: headers, Body: body, RetainBody: retainBody}, url.Display, nil
}
