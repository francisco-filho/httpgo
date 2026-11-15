package runner

import (
	"slices"
	"strings"

	"capimgrosso.com/httpgo/internal/assertion"
	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// HeaderFailure is sanitized before collection or delivery to the reporter.
type HeaderFailure struct {
	Position         parser.Position
	Name             string
	Operator         assertion.HeaderOperator
	Expected, Actual string
	Present          bool
}

func sensitiveHeader(name string) bool {
	switch strings.ToLower(name) {
	case "authorization", "proxy-authorization", "cookie", "set-cookie":
		return true
	default:
		return false
	}
}

func checkHeaders(session template.Session, declarations []parser.HeaderAssertion, headers []query.HeaderValue) *HeaderFailure {
	for _, declaration := range declarations {
		actual, present := query.Header(headers, declaration.Name)
		if assertion.Header(declaration.Operator, declaration.Expected, actual, present) {
			continue
		}
		expected := declaration.Expected
		if sensitiveHeader(declaration.Name) {
			expected, actual = "[oculto]", "[oculto]"
		}
		pos := declaration.Position
		pos.File = session.Sanitize(pos.File)
		return &HeaderFailure{Position: pos, Name: session.Sanitize(declaration.Name), Operator: declaration.Operator, Expected: session.Sanitize(expected), Actual: session.Sanitize(actual), Present: present}
	}
	return nil
}

func safeEntry(session template.Session, entry parser.Entry) parser.Entry {
	// Reports need metadata, not request bodies or raw response/header collections.
	entry.Body = nil
	entry.BodyPlan = template.JSONPlan{}
	entry.Headers = nil
	entry.Assertions = slices.Clone(entry.Assertions)
	for i := range entry.Assertions {
		declaration := &entry.Assertions[i]
		if sensitiveHeader(declaration.Name) {
			declaration.Expected = "[oculto]"
		} else {
			declaration.Expected = session.Sanitize(declaration.Expected)
		}
		declaration.Name = session.Sanitize(declaration.Name)
		declaration.Position.File = session.Sanitize(declaration.Position.File)
	}
	entry.URL = session.Sanitize(entry.URL)
	entry.Position.File = session.Sanitize(entry.Position.File)
	entry.ExpectedPosition.File = session.Sanitize(entry.ExpectedPosition.File)
	return entry
}
