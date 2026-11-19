package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/suite"
)

// progressDiagnostic uses only the runner's safe presentation metadata. Never
// reconstruct a declaration from scenario source or inspect response contents.
type progressDiagnostic struct {
	position                   parser.Position
	phase, expression, message string
}

func (p *fileProgress) diagnostic(run runner.Result, loadErr error) error {
	diagnostic, ok := p.command.progressDiagnostic(run, loadErr)
	if !ok {
		return nil
	}
	ordinal := p.done
	if loadErr != nil {
		ordinal = 0
	}
	return p.command.writeProgressDiagnostic(diagnostic, ordinal)
}

func (c *Command) writeProgressDiagnostic(diagnostic progressDiagnostic, ordinal int) error {
	var text strings.Builder
	location := ""
	if ordinal > 0 {
		location = fmt.Sprintf("step %d", ordinal)
	}
	if diagnostic.position.Line > 0 {
		if location != "" {
			location += " - "
		}
		location += fmt.Sprintf("Linha %d", diagnostic.position.Line)
		if diagnostic.position.Column > 0 {
			location += fmt.Sprintf(":%d", diagnostic.position.Column)
		}
	}
	if location != "" {
		fmt.Fprintf(&text, "  %s\n", location)
	}
	fmt.Fprintf(&text, "  [%s]\n", diagnostic.phase)
	if diagnostic.expression != "" {
		fmt.Fprintf(&text, "  %s\n", diagnostic.expression)
	}
	if c.color {
		fmt.Fprintf(&text, "     \x1b[31m%s\x1b[0m\n", diagnostic.message)
	} else {
		fmt.Fprintf(&text, "     %s\n", diagnostic.message)
	}
	_, err := io.WriteString(c.err, text.String())
	return err
}

func (c *Command) progressDiagnostic(run runner.Result, loadErr error) (progressDiagnostic, bool) {
	if loadErr != nil {
		diagnostic := progressDiagnostic{phase: "parsing", message: loadErr.Error()}
		var parseErr *parser.Error
		var fileErr *suite.Error
		switch {
		case errors.As(loadErr, &parseErr):
			diagnostic.position = parseErr.Position
			diagnostic.message = parseErr.Category + ": " + parseErr.Detail
		case errors.As(loadErr, &fileErr):
			diagnostic.message = fileErr.Detail
		}
		return diagnostic, true
	}
	var entry runner.EntryResult
	if len(run.Entries) > 0 {
		entry = run.Entries[len(run.Entries)-1]
	}
	if run.Err != nil {
		return c.progressExecutionError(run, entry), true
	}
	if !run.Diverged {
		return progressDiagnostic{}, false
	}
	diagnostic := progressDiagnostic{phase: "assertion", position: entry.Entry.Position}
	switch {
	case entry.JSONFailure != nil:
		failure := entry.JSONFailure
		diagnostic.position = failure.Position
		diagnostic.expression = jsonProgressExpression(failure.Query, string(failure.Operator), failure.Count)
		actual := jsonProgressActual(failure.Actual, failure.Present)
		if failure.Operator.TypePredicate() {
			diagnostic.message = "received " + actual + "; condição falsa"
		} else {
			diagnostic.message = fmt.Sprintf("expected %s => received %s", failure.Expected, actual)
		}
		if failure.Detail != "" {
			diagnostic.message += "; " + failure.Detail
		}
	case entry.HeaderFailure != nil:
		failure := entry.HeaderFailure
		diagnostic.position = failure.Position
		diagnostic.expression = fmt.Sprintf("header %q %s", failure.Name, failure.Operator)
		expected := fmt.Sprintf("%q", failure.Expected)
		if failure.Operator == "exists" && failure.Expected != "[oculto]" {
			expected = "presente"
		}
		actual := "ausente"
		if failure.Present {
			actual = fmt.Sprintf("%q", failure.Actual)
		} else if failure.Actual == "[oculto]" {
			actual = "[oculto] (ausente)"
		}
		diagnostic.message = fmt.Sprintf("expected %s => received %s", expected, actual)
	case entry.BodyFailure != nil:
		failure := entry.BodyFailure
		diagnostic.position = failure.Position
		diagnostic.expression = "body " + string(failure.Operator)
		diagnostic.message = fmt.Sprintf("expected %q => condição falsa", failure.Expected)
	case entry.RegexFailure != nil:
		failure := entry.RegexFailure
		diagnostic.position = failure.Position
		diagnostic.expression = fmt.Sprintf("regex %q %s", failure.Pattern, failure.Operator)
		actual := "grupo presente; condição falsa"
		if !failure.Present {
			actual = "grupo ausente"
		}
		diagnostic.message = fmt.Sprintf("expected %q => received %s", failure.Expected, actual)
	default:
		diagnostic.phase = "status"
		diagnostic.position = entry.Entry.ExpectedPosition
		diagnostic.message = "status divergente"
		if entry.Entry.ExpectedStatus != nil {
			expected := *entry.Entry.ExpectedStatus
			diagnostic.expression = fmt.Sprintf("HTTP %d", expected)
			diagnostic.message = fmt.Sprintf("expected %d => received %d", expected, entry.Status)
		}
	}
	return diagnostic, true
}

func jsonProgressExpression(query, operator string, count bool) string {
	expression := fmt.Sprintf("jsonpath %q", query)
	if count {
		expression += " count"
	}
	if operator != "" {
		expression += " " + operator
	}
	return expression
}

func jsonProgressActual(actual string, present bool) string {
	if !present {
		if actual == "[oculto]" {
			return "[oculto] (ausente)"
		}
		return "ausente"
	}
	// Keep the compact type description, not the optional collection count.
	if strings.HasPrefix(actual, "tipo array; contagem=") {
		return "tipo array"
	}
	return actual
}

func (c *Command) progressExecutionError(run runner.Result, entry runner.EntryResult) progressDiagnostic {
	diagnostic := progressDiagnostic{position: run.ErrorPosition, phase: "transporte", message: transportDiagnostic(run.Err, c.timeout)}
	var renderErr *runner.RenderError
	var fileErr *runner.FileError
	var captureErr *runner.CaptureError
	var jsonErr *runner.JSONError
	var headerErr *runner.HeaderError
	var bodyErr *runner.BodyError
	var regexErr *runner.RegexError
	switch {
	case errors.As(run.Err, &renderErr):
		diagnostic.phase, diagnostic.position, diagnostic.message = "renderização", renderErr.Position, renderErr.Error()
	case errors.As(run.Err, &fileErr):
		diagnostic.phase, diagnostic.position, diagnostic.message = "renderização", fileErr.Position, fileErr.Error()
	case errors.As(run.Err, &captureErr):
		diagnostic.phase, diagnostic.position, diagnostic.message = "captures", captureErr.Position, captureErr.Detail
		if captureErr.Query != "" {
			diagnostic.expression = jsonProgressExpression(captureErr.Query, "", captureErr.Count)
		}
	case errors.As(run.Err, &jsonErr):
		diagnostic.phase, diagnostic.position, diagnostic.message = "assertion", jsonErr.Position, jsonErr.Detail
		diagnostic.expression = assertionProgressExpression(entry, jsonErr.Position)
		if diagnostic.expression == "" && jsonErr.Query != "" {
			diagnostic.expression = jsonProgressExpression(jsonErr.Query, "", false)
		}
	case errors.As(run.Err, &headerErr):
		diagnostic.phase, diagnostic.position, diagnostic.message = "assertion", headerErr.Position, headerErr.Detail
		diagnostic.expression = assertionProgressExpression(entry, headerErr.Position)
		if diagnostic.expression == "" {
			diagnostic.expression = fmt.Sprintf("header %q", headerErr.Name)
		}
	case errors.As(run.Err, &bodyErr):
		diagnostic.phase, diagnostic.position, diagnostic.message = "assertion", bodyErr.Position, bodyErr.Detail
		diagnostic.expression = "body " + string(bodyErr.Operator)
	case errors.As(run.Err, &regexErr):
		diagnostic.phase, diagnostic.position, diagnostic.message = "assertion", regexErr.Position, regexErr.Detail
		// Safe entry metadata intentionally omits regex source on evaluation errors.
		diagnostic.expression = "regex " + string(regexErr.Operator)
	}
	return diagnostic
}

func assertionProgressExpression(entry runner.EntryResult, pos parser.Position) string {
	for _, declaration := range entry.Entry.Assertions {
		switch {
		case declaration.JSON != nil:
			item := declaration.JSON
			if item.Position == pos || item.QueryPosition == pos {
				return jsonProgressExpression(item.Query, string(item.Operator), item.Count)
			}
		case declaration.Header != nil:
			item := declaration.Header
			if item.Position == pos {
				return fmt.Sprintf("header %q %s", item.Name, item.Operator)
			}
		}
	}
	return ""
}
