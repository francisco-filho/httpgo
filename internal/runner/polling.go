package runner

import (
	"context"
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

func (r *Runner) RunWithOptions(ctx context.Context, scenario parser.Scenario, options Options) Result {
	result := Result{Summary: Summary{Total: len(scenario.Entries), NotStarted: len(scenario.Entries)}}
	session := r.renderer.NewSession()
	protectScenario(session, scenario)
	for _, entry := range scenario.Entries {
		if err := ctx.Err(); err != nil {
			result.Err, result.ErrorPosition = err, entry.Position
			return result
		}
		begin := r.now()
		execution := r.pollEntry(ctx, scenario, entry, options, session)
		duration := max(time.Duration(0), r.now().Sub(begin))
		for i := range execution.Entries {
			execution.Entries[i].Duration = duration
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

func (r *Runner) pollEntry(ctx context.Context, scenario parser.Scenario, entry parser.Entry, options Options, session template.Session) Result {
	var metrics AttemptSummary
	facts := Summary{Total: 1, NotStarted: 1}
	for remaining := entry.Options.Retry; ; remaining-- {
		current := r.runAttempt(ctx, scenario.OriginDir, entry, options, session)
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
