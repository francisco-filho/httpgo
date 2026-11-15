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

func checkHeader(session template.Session, declaration parser.HeaderAssertion, headers []query.HeaderValue, conceal bool) *HeaderFailure {
	actual, present := query.Header(headers, declaration.Name)
	if assertion.Header(declaration.Operator, declaration.Expected, actual, present) {
		return nil
	}
	expected := declaration.Expected
	if conceal || sensitiveHeader(declaration.Name) {
		expected, actual = "[oculto]", "[oculto]"
	}
	pos := declaration.Position
	pos.File = session.Sanitize(pos.File)
	return &HeaderFailure{Position: pos, Name: session.Sanitize(declaration.Name), Operator: declaration.Operator, Expected: session.Sanitize(expected), Actual: session.Sanitize(actual), Present: present}
}

func pendingSecret(session template.Session, entry parser.Entry) bool {
	for _, capture := range entry.Captures {
		if session.IsSecret(capture.Name) {
			return true
		}
	}
	return false
}

func safeEntry(session template.Session, entry parser.Entry) parser.Entry {
	// Reports need metadata, not request bodies or raw response/header collections.
	entry.Multipart = nil
	entry.ExternalBody = nil
	entry.Body = nil
	entry.BodyPlan = template.JSONPlan{}
	entry.Headers = nil
	entry.HeaderPositions = nil
	entry.BodyPosition.File = session.Sanitize(entry.BodyPosition.File)
	entry.URLPosition.File = session.Sanitize(entry.URLPosition.File)
	conceal := pendingSecret(session, entry)
	entry.Captures = nil // Compiled queries are not report metadata.
	entry.Assertions = slices.Clone(entry.Assertions)
	for i := range entry.Assertions {
		if entry.Assertions[i].JSON != nil {
			declaration := *entry.Assertions[i].JSON
			declaration.Expected = nil // Never deliver the raw literal to reports.
			declaration.Path = query.Path{}
			declaration.Query = session.Sanitize(declaration.Query)
			declaration.Position.File = session.Sanitize(declaration.Position.File)
			declaration.QueryPosition.File = session.Sanitize(declaration.QueryPosition.File)
			entry.Assertions[i].JSON = &declaration
			continue
		}
		if entry.Assertions[i].Header == nil {
			continue
		}
		copy := *entry.Assertions[i].Header
		entry.Assertions[i].Header = &copy
		declaration := &copy
		if conceal || sensitiveHeader(declaration.Name) {
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
