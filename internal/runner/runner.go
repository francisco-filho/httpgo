// Package runner executes validated scenarios sequentially, stopping at failure.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"capimgrosso.com/httpgo/internal/assertion"
	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

type Request struct {
	Timeout          time.Duration // Zero uses the executor's default; one budget for all redirects and response I/O.
	Multipart        *MultipartBody
	Method           string
	URL              string
	Headers          []parser.Header
	Body             []byte
	RetainBody       bool
	DiagnosticBodies bool // Opt-in final-hop diagnostic data, never report data.
}
type Response struct {
	Status      int
	Body        []byte
	Headers     []query.HeaderValue
	Diagnostics *HTTPDiagnostics // Transient operational data for the runner only.
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
	Cancelled      Outcome = "execução cancelada"
	RenderFailed   Outcome = "erro de renderização"
	FileFailed     Outcome = "erro de arquivo externo"
	CaptureFailed  Outcome = "erro de captura"
	HeaderMismatch Outcome = "assertion de header divergente"
	HeaderFailed   Outcome = "erro de avaliação de header"
	JSONMismatch   Outcome = "assertion JSON divergente"
	JSONFailed     Outcome = "erro de avaliação JSON"
	BodyMismatch   Outcome = "assertion de body divergente"
	BodyFailed     Outcome = "erro de avaliação de body"
	RegexMismatch  Outcome = "assertion regex divergente"
	RegexFailed    Outcome = "erro de avaliação regex"
)

// EntryDiagnostic carries only safe terminal error metadata, never raw causes.
type EntryDiagnostic struct {
	Position             parser.Position
	Phase, Detail, Query string
	Count                bool
}

type EntryResult struct {
	Diagnostic    *EntryDiagnostic
	Bodies        *BodyPreviews // Protected, bounded previews on terminal failures only.
	Duration      time.Duration // Logical entry, including preparation, attempts and waits; excludes reporting.
	Attempts      int
	RegexFailure  *RegexFailure
	RegexError    *RegexError
	BodyFailure   *BodyFailure
	BodyError     *BodyError
	JSONFailure   *JSONFailure
	JSONError     *JSONError
	HeaderFailure *HeaderFailure
	HeaderError   *HeaderError
	Entry         parser.Entry
	Outcome       Outcome
	Status        int
	DisplayURL    string
}

type Summary struct {
	Total, Started, Completed, Validated, WithoutValidation, NotStarted int
}

// AttemptSummary counts executor calls, complete responses and status comparisons.
// Redirect hops are internal to one executor call, not additional attempts.
type AttemptSummary struct {
	Started, Completed, StatusChecks int
}

type Result struct {
	Attempts      AttemptSummary
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
	Position  parser.Position
	Reference *parser.Position // Scenario reference for an external payload, when present.
	Detail    string
	cause     error
}

func (e *RenderError) Error() string {
	diagnostic := fmt.Sprintf("erro de renderização: %s", e.Detail)
	if e.Reference != nil {
		pos := e.Reference
		diagnostic += fmt.Sprintf(" (referência %q:%d:%d)", pos.File, pos.Line, pos.Column)
	}
	return diagnostic
}
func (e *RenderError) Unwrap() error { return e.cause }

type Runner struct {
	executor   HTTPExecutor
	report     func(EntryResult) error
	renderer   Renderer
	readFile   func(context.Context, string) ([]byte, error)
	openUpload func(string) (uploadFile, error)
	wait       func(context.Context, time.Duration) error
	now        func() time.Time
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
	return &Runner{executor: executor, renderer: renderer, report: report, readFile: readPayload, openUpload: openUpload, wait: waitInterval, now: time.Now}
}

// Options applies to one execution only. The zero value preserves external bytes.
type Options struct {
	Timeout          time.Duration // Zero preserves the default ten-second transport policy.
	RenderFiles      bool
	RenderAsserts    bool // Used by suite loading to select the matching parser profile.
	DiagnosticBodies bool // Retain final bodies transiently to prepare safe failure previews.
}

func (r *Runner) Run(ctx context.Context, scenario parser.Scenario) Result {
	return r.RunWithOptions(ctx, scenario, Options{})
}

func protectScenario(session template.Session, scenario parser.Scenario) {
	for _, entry := range scenario.Entries {
		for _, capture := range entry.Captures {
			if capture.Kind == parser.HeaderCapture && sensitiveHeader(capture.Header) {
				session.MarkSecret(capture.Name)
			}
			if session.IsSecret(capture.Name) && capture.Compiled != nil {
				session.ProtectValue(capture.Pattern)
			}
		}
		for _, declaration := range entry.Assertions {
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
	// Collect after all static secret declarations, including sensitive captures
	// in later entries. Protection does not make future values resolvable and
	// must include discarded declarations, not just the effective batch.
	for _, entry := range scenario.Entries {
		for _, declaration := range entry.Options.Variables {
			session.ProtectCapture(declaration.Name, declaration.Value)
		}
	}
}

// runAttempt executes one entry without reporting. Resources are closed before
// evaluation or waiting; only the polling decision publishes a terminal result.
func (r *Runner) runAttempt(ctx context.Context, originDir string, entry parser.Entry, options Options, session template.Session, last **attemptBodies) Result {
	result := Result{Summary: Summary{Total: 1, NotStarted: 1}}
	if err := ctx.Err(); err != nil {
		result.Err, result.ErrorPosition = err, entry.Position
		return result
	}
	request, display, renderErr := render(session, entry)
	if renderErr != nil {
		result.Err, result.ErrorPosition = renderErr, renderErr.Position
		current := EntryResult{Entry: safeEntry(session, entry), Outcome: RenderFailed, DisplayURL: "[URL não renderizada]"}
		result.Entries = append(result.Entries, current)
		return result
	}
	for _, header := range request.Headers {
		if sensitiveHeader(header.Name) {
			session.ProtectValue(header.Value)
		}
	}
	if entry.ExternalBody != nil {
		path := entry.ExternalBody.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(originDir, path)
		}
		body, err := r.readFile(ctx, path)
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			failure := fileFailure(entry.ExternalBody.Position, err)
			result.Err, result.ErrorPosition = failure, failure.Position
			current := EntryResult{Entry: safeEntry(session, entry), Outcome: FileFailed, DisplayURL: session.Sanitize(display)}
			current.Entry.Position = failure.Position
			current.Entry.Position.File = session.Sanitize(current.Entry.Position.File)
			result.Entries = append(result.Entries, current)
			return result
		}
		if options.RenderFiles {
			var failure *RenderError
			body, failure = renderExternal(session, body, *entry.ExternalBody)
			if err := ctx.Err(); err != nil {
				result.Err, result.ErrorPosition = err, entry.ExternalBody.Position
				return result
			}
			if failure != nil {
				result.Err, result.ErrorPosition = failure, failure.Position
				current := EntryResult{Entry: safeEntry(session, entry), Outcome: RenderFailed, DisplayURL: "[URL não renderizada]"}
				current.Entry.Position = failure.Position
				result.Entries = append(result.Entries, current)
				return result
			}
		}
		request.Body = body
	}
	var upload io.Closer
	if entry.Multipart != nil {
		var err error
		upload, err = r.prepareMultipart(ctx, session, entry, originDir, &request)
		if err != nil {
			pos, outcome := entry.Multipart.Position, FileFailed
			var fileErr *FileError
			var renderErr *RenderError
			if errors.As(err, &fileErr) {
				pos = fileErr.Position
			}
			if errors.As(err, &renderErr) {
				pos, outcome = renderErr.Position, RenderFailed
			}
			result.Err, result.ErrorPosition = err, pos
			current := EntryResult{Entry: safeEntry(session, entry), Outcome: outcome, DisplayURL: session.Sanitize(display)}
			current.Entry.Position = pos
			current.Entry.Position.File = session.Sanitize(pos.File)
			result.Entries = append(result.Entries, current)
			return result
		}
	}
	if err := ctx.Err(); err != nil {
		if upload != nil {
			err = errors.Join(err, upload.Close())
		}
		result.Err, result.ErrorPosition = err, entry.Position
		return result
	}
	request.Timeout = options.Timeout
	request.DiagnosticBodies = options.DiagnosticBodies
	result.Summary.Started++
	result.Summary.NotStarted--
	response, err := r.executor.Execute(ctx, request)
	if options.DiagnosticBodies {
		*last = newAttemptBodies(response.Diagnostics, session, entry)
	}
	if upload != nil {
		if closeErr := upload.Close(); closeErr != nil {
			err = errors.Join(err, fileFailure(entry.Multipart.Position, closeErr))
		}
	}
	for _, header := range response.Headers {
		if sensitiveHeader(header.Name) {
			session.ProtectValue(header.Value)
		}
	}
	current := EntryResult{Entry: entry, Status: response.Status, DisplayURL: display}
	if err != nil {
		current.Outcome, current.Status = Failed, 0
		result.Err, result.ErrorPosition = err, entry.Position
		var fileErr *FileError
		if errors.As(err, &fileErr) {
			result.ErrorPosition = fileErr.Position
		}
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
	view := session
	var batch []template.Capture
	if err == nil && !result.Diverged && len(entry.Captures) > 0 {
		var captureErr *CaptureError
		batch, captureErr = extractCaptures(session, entry.Captures, response, func(index int) {
			if options.DiagnosticBodies {
				delete((*last).pending, index)
			}
		})
		if captureErr == nil {
			var stageErr error
			view, stageErr = session.Stage(batch)
			if stageErr != nil {
				captureErr = captureBatchFailure(session, entry.Captures, stageErr)
			}
		}
		if captureErr != nil {
			current.Outcome = CaptureFailed
			result.Err, result.ErrorPosition = captureErr, captureErr.Position
		}
	}
	if err == nil && result.Err == nil && !result.Diverged {
		if assertionErr := checkAssertions(view, entry, response, &current); assertionErr != nil {
			result.Err = assertionErr
			var jsonErr *JSONError
			var bodyErr *BodyError
			var headerErr *HeaderError
			var regexErr *RegexError
			switch {
			case errors.As(assertionErr, &regexErr):
				result.ErrorPosition = regexErr.Position
			case errors.As(assertionErr, &headerErr):
				result.ErrorPosition = headerErr.Position
			case errors.As(assertionErr, &jsonErr):
				current.JSONError = jsonErr
				result.ErrorPosition = jsonErr.Position
			case errors.As(assertionErr, &bodyErr):
				result.ErrorPosition = bodyErr.Position
			default:
				result.ErrorPosition = entry.Position
			}
		}
		result.Diverged = current.HeaderFailure != nil || current.JSONFailure != nil || current.BodyFailure != nil || current.RegexFailure != nil
	}
	if result.Err == nil {
		if cancelErr := ctx.Err(); cancelErr != nil {
			current.Outcome = Cancelled
			result.Err, result.ErrorPosition, result.Diverged = cancelErr, entry.Position, false
		} else if !result.Diverged && len(batch) > 0 {
			if publishErr := session.Publish(batch); publishErr != nil {
				captureErr := captureBatchFailure(session, entry.Captures, publishErr)
				current.Outcome = CaptureFailed
				result.Err, result.ErrorPosition = captureErr, captureErr.Position
			}
		}
	}
	current.Entry = safeEntry(session, entry)
	current.DisplayURL = session.Sanitize(current.DisplayURL)
	result.Entries = append(result.Entries, current)
	return result
}
