package runner

import (
	"context"
	"errors"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

func waitInterval(ctx context.Context, interval time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if interval == 0 {
		return nil
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

// reservedDeclaration reports the first capture or [Options] variable that uses
// an exact built-in function name, before any entry is prepared or executed.
func reservedDeclaration(scenario parser.Scenario) *parser.Position {
	for _, entry := range scenario.Entries {
		for _, capture := range entry.Captures {
			if template.ReservedFunction(capture.Name) {
				pos := capture.Position
				return &pos
			}
		}
		for _, declaration := range entry.Options.Variables {
			if template.ReservedFunction(declaration.Name) {
				pos := declaration.Position
				return &pos
			}
		}
	}
	return nil
}

func (r *Runner) RunWithOptions(ctx context.Context, scenario parser.Scenario, options Options) Result {
	result := Result{Summary: Summary{Total: len(scenario.Entries), NotStarted: len(scenario.Entries)}}
	session := r.renderer.NewSession()
	protectScenario(session, scenario)
	// Reserved function names are rejected before any entry, so no request is
	// ever sent with a declaration that could never be resolved.
	if reserved := reservedDeclaration(scenario); reserved != nil {
		failure := &RenderError{Position: *reserved, Detail: template.ReservedDetail}
		result.Err, result.ErrorPosition = failure, failure.Position
		return result
	}
	for _, entry := range scenario.Entries {
		if err := ctx.Err(); err != nil {
			result.Err, result.ErrorPosition = err, entry.Position
			return result
		}
		begin := r.now()
		execution := r.runEntry(ctx, scenario, entry, options, session)
		duration := max(time.Duration(0), r.now().Sub(begin))
		for i := range execution.Entries {
			execution.Entries[i].Duration = duration
			execution.Entries[i].Diagnostic = terminalDiagnostic(execution, session, options)
		}
		result.Attempts.Started += execution.Attempts.Started
		result.Attempts.Completed += execution.Attempts.Completed
		result.Attempts.StatusChecks += execution.Attempts.StatusChecks
		result.Summary.Started += execution.Summary.Started
		result.Summary.NotStarted -= execution.Summary.Started
		result.Summary.Completed += execution.Summary.Completed
		result.Summary.Validated += execution.Summary.Validated
		result.Summary.WithoutValidation += execution.Summary.WithoutValidation
		result.Err, result.ErrorPosition, result.Diverged = execution.Err, execution.ErrorPosition, execution.Diverged
		for _, current := range execution.Entries {
			result.Entries = append(result.Entries, current)
			if r.report != nil {
				if err := r.report(current); err != nil {
					result.ReportErr, result.ErrorPosition = err, entry.Position
					return result
				}
			}
		}
		if result.Err != nil || result.Diverged {
			return result
		}
	}
	return result
}

// runEntry publishes once, before preparing any polling attempt. Keep only the
// last declaration per name, in source order, without mutating the scenario.
func (r *Runner) runEntry(ctx context.Context, scenario parser.Scenario, entry parser.Entry, options Options, session template.Session) Result {
	declarations := entry.Options.Variables
	last := make(map[string]int, len(declarations))
	for i, declaration := range declarations {
		last[declaration.Name] = i
	}
	var batch []template.Capture
	var positions []parser.Position
	for i, declaration := range declarations {
		if last[declaration.Name] == i {
			batch = append(batch, template.Capture{Name: declaration.Name, Value: declaration.Value})
			positions = append(positions, declaration.Position)
		}
	}
	if err := ctx.Err(); err != nil {
		return Result{Err: err, ErrorPosition: entry.Position}
	}
	if len(batch) > 0 {
		if err := session.Publish(batch); err != nil {
			pos := positions[0]
			detail := "não foi possível atribuir variável de [Options]; verifique nomes em conflito ancestral/descendente"
			var conflict *template.ConflictError
			if errors.As(err, &conflict) && conflict.Index >= 0 && conflict.Index < len(positions) {
				pos = positions[conflict.Index]
			}
			var reserved *template.ReservedError
			if errors.As(err, &reserved) {
				if reserved.Index >= 0 && reserved.Index < len(positions) {
					pos = positions[reserved.Index]
				}
				detail = template.ReservedDetail
			}
			pos.File = session.Sanitize(pos.File)
			failure := &RenderError{Position: pos, Detail: detail, cause: err}
			return Result{Err: failure, ErrorPosition: pos, Entries: []EntryResult{{Entry: safeEntry(session, entry), Outcome: RenderFailed, DisplayURL: "[URL não renderizada]"}}}
		}
	}
	return r.pollEntry(ctx, scenario, entry, options, session)
}

// attemptlessResult shapes a failure that never reached an executor call: no
// attempt is invented and the entry keeps its terminal classification.
func attemptlessResult(session template.Session, entry parser.Entry, failure Result) Result {
	if len(failure.Entries) == 0 && failure.Err != nil {
		failure.Entries = []EntryResult{{Entry: safeEntry(session, entry), Outcome: Cancelled}}
	}
	failure.Summary = Summary{Total: 1, NotStarted: 1}
	for i := range failure.Entries {
		failure.Entries[i].Attempts = 0
	}
	return failure
}

func (r *Runner) pollEntry(ctx context.Context, scenario parser.Scenario, entry parser.Entry, options Options, session template.Session) (result Result) {
	var last *attemptBodies
	defer func() {
		if options.DiagnosticBodies && (result.Err != nil || result.Diverged) {
			for i := range result.Entries {
				result.Entries[i].Bodies = prepareBodyPreviews(last, session)
			}
		}
	}()
	if err := ctx.Err(); err != nil {
		return attemptlessResult(session, entry, Result{Err: err, ErrorPosition: entry.Position})
	}
	prepared, failure := prepareEntry(session, entry)
	// Cancellation during preparation is terminal, not a render failure.
	if err := ctx.Err(); err != nil {
		return attemptlessResult(session, entry, Result{Err: err, ErrorPosition: entry.Position})
	}
	if failure != nil {
		current := Result{Err: failure, ErrorPosition: failure.Position, Entries: []EntryResult{{Entry: safeEntry(session, entry), Outcome: RenderFailed, DisplayURL: "[URL não renderizada]"}}}
		return attemptlessResult(session, entry, current)
	}
	var metrics AttemptSummary
	facts := Summary{Total: 1, NotStarted: 1}
	for remaining := entry.Options.Retry; ; remaining-- {
		current := r.runAttempt(ctx, scenario.OriginDir, entry, options, session, prepared, &last)
		metrics.Started += current.Summary.Started
		metrics.Completed += current.Summary.Completed
		if current.Summary.Completed != 0 && entry.ExpectedStatus != nil {
			metrics.StatusChecks++
		}
		if current.Summary.Started != 0 {
			facts = current.Summary
		}
		current.Summary, current.Attempts = facts, metrics
		if len(current.Entries) == 0 && current.Err != nil {
			current.Entries = []EntryResult{{Entry: safeEntry(session, entry), Outcome: Cancelled}}
		}
		for i := range current.Entries {
			current.Entries[i].Attempts = metrics.Started
		}
		// Only classified assertion/status divergence is eligible; every category 2
		// terminates, including capture/preparation failures and cancellation.
		if current.Err != nil || !current.Diverged || remaining == 0 {
			return current
		}
		if err := r.wait(ctx, entry.Options.Interval()); err != nil {
			current.Err, current.ErrorPosition, current.Diverged = err, entry.Position, false
			current.Entries = []EntryResult{{Entry: safeEntry(session, entry), Outcome: Cancelled, DisplayURL: current.Entries[0].DisplayURL, Attempts: metrics.Started}}
			return current
		}
	}
}
