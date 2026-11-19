package runner

import (
	"bytes"
	"encoding/json"
	"mime"
	"strings"
	"unicode/utf8"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

const bodyPreviewLimit = 16 * 1024

type BodyState string

const (
	BodyAbsent      BodyState = "ausente"
	BodyUnavailable BodyState = "indisponível/incompleto"
	BodyOmitted     BodyState = "omitido (conteúdo não JSON)"
	BodyProtected   BodyState = "omitido por proteção"
	BodyJSON        BodyState = "json"
)

// HTTPBody and HTTPDiagnostics are operational executor data. They must never
// reach callbacks or result collections; only BodyPreviews may leave the runner.
type HTTPBody struct {
	Bytes       []byte
	State       BodyState
	ContentType string
}

type HTTPDiagnostics struct{ Request, Response HTTPBody }

type BodyPreview struct {
	State     BodyState
	Text      string
	Truncated bool
}

type BodyPreviews struct{ Request, Response BodyPreview }

type attemptBodies struct {
	data    HTTPDiagnostics
	pending map[string]bool
}

func newAttemptBodies(data *HTTPDiagnostics, session template.Session, entry parser.Entry) *attemptBodies {
	result := &attemptBodies{data: HTTPDiagnostics{Request: HTTPBody{State: BodyUnavailable}, Response: HTTPBody{State: BodyUnavailable}}, pending: make(map[string]bool)}
	if data != nil {
		result.data = *data
	}
	for _, capture := range entry.Captures {
		if session.IsSecret(capture.Name) || capture.Path.HasProtectedReference(session.IsSecret) {
			result.pending[capture.Name] = true
		}
	}
	return result
}

func prepareBodyPreviews(last *attemptBodies, session template.Session) *BodyPreviews {
	if last == nil {
		return &BodyPreviews{Request: BodyPreview{State: BodyUnavailable}, Response: BodyPreview{State: BodyUnavailable}}
	}
	result := &BodyPreviews{Request: prepareBodyPreview(last.data.Request, session)}
	if len(last.pending) > 0 {
		result.Response.State = BodyProtected
	} else {
		result.Response = prepareBodyPreview(last.data.Response, session)
	}
	return result
}

func prepareBodyPreview(body HTTPBody, session template.Session) BodyPreview {
	switch body.State {
	case BodyAbsent, BodyUnavailable, BodyOmitted, BodyProtected:
		return BodyPreview{State: body.State}
	case BodyJSON:
		// This state means a complete candidate, not that the executor evaluated JSON.
	default:
		return BodyPreview{State: BodyUnavailable}
	}
	if !diagnosticJSONType(body.ContentType) || !utf8.Valid(body.Bytes) || !json.Valid(body.Bytes) {
		return BodyPreview{State: BodyOmitted}
	}
	text := string(body.Bytes)
	// Normalize only when necessary to protect semantically equivalent JSON escape
	// spellings (e.g. a secret encoded as \u0061). Keep original bytes otherwise.
	decoder := json.NewDecoder(bytes.NewReader(body.Bytes))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return BodyPreview{State: BodyOmitted}
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return BodyPreview{State: BodyOmitted}
	}
	if safe := session.Sanitize(string(canonical)); safe != string(canonical) {
		text = safe
	} else {
		text = session.Sanitize(text)
	}
	// JSON permits CR whitespace, but static terminal output must not contain
	// carriage-return updates. Escaped CR inside strings remains unchanged.
	text = strings.ReplaceAll(text, "\r", "")
	result := BodyPreview{State: BodyJSON}
	if len(text) > bodyPreviewLimit {
		text = text[:bodyPreviewLimit]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
		result.Truncated = true
	}
	result.Text = text
	return result
}

func diagnosticJSONType(contentType string) bool {
	if contentType == "" {
		return true
	}
	media, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	charset := strings.ToLower(params["charset"])
	if charset != "" && charset != "utf-8" && charset != "utf8" {
		return false
	}
	return media == "application/json" || strings.HasSuffix(media, "+json") || media == "text/plain"
}
