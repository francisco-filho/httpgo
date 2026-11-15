// Package runner executes validated scenarios sequentially, stopping at failure.
package runner

import (
	"context"
	"fmt"

	"capimgrosso.com/httpgo/internal/assertion"
	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

type Request struct {
	Method     string
	URL        string
	Headers    []parser.Header
	Body       []byte
	RetainBody bool
}
type Response struct {
	Status  int
	Body    []byte
	Headers []query.HeaderValue
}

// HTTPExecutor returns a response only after consuming and closing its body.
type HTTPExecutor interface {
	Execute(context.Context, Request) (Response, error)
}

type Outcome string

const (
	Wildcard       Outcome = "status aceito por HTTP *"
	Unvalidated    Outcome = "sem validação de status"
	Passed         Outcome = "status validado"
	Mismatch       Outcome = "status divergente"
	Failed         Outcome = "erro de transporte"
	RenderFailed   Outcome = "erro de renderização"
	CaptureFailed  Outcome = "erro de captura"
	HeaderMismatch Outcome = "assertion de header divergente"
)

type EntryResult struct {
	HeaderFailure *HeaderFailure
	Entry         parser.Entry
	Outcome       Outcome
	Status        int
	DisplayURL    string
}

type Summary struct {
	Total, Started, Completed, Validated, WithoutValidation, NotStarted int
}

type Result struct {
	Entries       []EntryResult
	Summary       Summary
	Err           error
	ReportErr     error
	ErrorPosition parser.Position
	Diverged      bool
}

// Renderer creates an isolated session for each Run.
type Renderer interface {
	NewSession() template.Session
}

type RenderError struct {
	Position parser.Position
	Detail   string
	cause    error
}

func (e *RenderError) Error() string { return fmt.Sprintf("erro de renderização: %s", e.Detail) }
func (e *RenderError) Unwrap() error { return e.cause }

type Runner struct {
	executor HTTPExecutor
	report   func(EntryResult) error
	renderer Renderer
}

// New accepts an optional synchronous reporter, called before the next entry.
// A reporting error stops execution; nil collects results without reporting.
func New(executor HTTPExecutor, report func(EntryResult) error) *Runner {
	return NewWithRenderer(executor, nil, report)
}

func NewWithRenderer(executor HTTPExecutor, renderer Renderer, report func(EntryResult) error) *Runner {
	if renderer == nil {
		renderer, _ = template.New(nil, nil)
	}
	return &Runner{executor: executor, renderer: renderer, report: report}
}

func (r *Runner) Run(ctx context.Context, scenario parser.Scenario) Result {
	result := Result{Summary: Summary{Total: len(scenario.Entries), NotStarted: len(scenario.Entries)}}
	session := r.renderer.NewSession()
	for _, entry := range scenario.Entries {
		for _, declaration := range entry.Assertions {
			if declaration.JSON != nil {
				result.Err = fmt.Errorf("execução de assertion JSON ainda não suportada")
				result.ErrorPosition = declaration.JSON.Position
				return result
			}
			if declaration.Header != nil && sensitiveHeader(declaration.Header.Name) {
				session.ProtectValue(declaration.Header.Expected)
			}
		}
		for _, header := range entry.Headers {
			if sensitiveHeader(header.Name) {
				session.ProtectValue(header.Value)
			}
		}
	}
	for _, entry := range scenario.Entries {
		if err := ctx.Err(); err != nil {
			result.Err, result.ErrorPosition = err, entry.Position
			return result
		}
		request, display, renderErr := render(session, entry)
		if renderErr != nil {
			result.Err, result.ErrorPosition = renderErr, renderErr.Position
			current := EntryResult{Entry: safeEntry(session, entry), Outcome: RenderFailed, DisplayURL: "[URL não renderizada]"}
			result.Entries = append(result.Entries, current)
			if r.report != nil {
				if err := r.report(current); err != nil {
					result.ReportErr = err
				}
			}
			return result
		}
		result.Summary.Started++
		result.Summary.NotStarted--
		for _, header := range request.Headers {
			if sensitiveHeader(header.Name) {
				session.ProtectValue(header.Value)
			}
		}
		response, err := r.executor.Execute(ctx, request)
		for _, header := range response.Headers {
			if sensitiveHeader(header.Name) {
				session.ProtectValue(header.Value)
			}
		}
		current := EntryResult{Entry: entry, Status: response.Status, DisplayURL: display}
		if err != nil {
			current.Outcome, current.Status = Failed, 0
			result.Err, result.ErrorPosition = err, entry.Position
		} else {
			result.Summary.Completed++
			if entry.ExpectedStatus == nil {
				current.Outcome = Unvalidated
				if entry.AnyStatus {
					current.Outcome = Wildcard
				}
				result.Summary.WithoutValidation++
			} else if assertion.Status(*entry.ExpectedStatus, response.Status) {
				current.Outcome = Passed
				result.Summary.Validated++
			} else {
				current.Outcome = Mismatch
				result.Diverged = true
			}
		}
		if err == nil && !result.Diverged {
			current.HeaderFailure = checkHeaders(session, entry.Assertions, response.Headers)
			if current.HeaderFailure != nil {
				current.Outcome, result.Diverged = HeaderMismatch, true
			}
		}
		if err == nil && !result.Diverged && len(entry.Captures) > 0 {
			if captureErr := captureResponse(session, entry.Captures, response.Body); captureErr != nil {
				current.Outcome = CaptureFailed
				result.Err, result.ErrorPosition = captureErr, captureErr.Position
			}
		}
		current.Entry = safeEntry(session, entry)
		current.DisplayURL = session.Sanitize(current.DisplayURL)
		result.Entries = append(result.Entries, current)
		if r.report != nil {
			if err := r.report(current); err != nil {
				result.ReportErr, result.ErrorPosition = err, entry.Position
				return result
			}
		}
		if result.Err != nil || result.Diverged {
			return result
		}
	}
	return result
}
