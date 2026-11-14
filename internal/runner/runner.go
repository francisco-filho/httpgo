// Package runner executes validated scenarios sequentially, stopping at failure.
package runner

import (
	"context"

	"capimgrosso.com/httpgo/internal/assertion"
	"capimgrosso.com/httpgo/internal/parser"
)

type Request struct{ URL string }
type Response struct{ Status int }

// HTTPExecutor returns a response only after consuming and closing its body.
type HTTPExecutor interface {
	Execute(context.Context, Request) (Response, error)
}

type Outcome string

const (
	Unvalidated Outcome = "sem validação de status"
	Passed      Outcome = "status validado"
	Mismatch    Outcome = "status divergente"
	Failed      Outcome = "erro de transporte"
)

type EntryResult struct {
	Entry   parser.Entry
	Outcome Outcome
	Status  int
}

type Summary struct {
	Total, Started, Completed, Validated, WithoutValidation, NotStarted int
}

type Result struct {
	Entries       []EntryResult
	Summary       Summary
	Err           error
	ErrorPosition parser.Position
	Diverged      bool
}

type Runner struct{ executor HTTPExecutor }

func New(executor HTTPExecutor) *Runner { return &Runner{executor: executor} }

func (r *Runner) Run(ctx context.Context, scenario parser.Scenario) Result {
	result := Result{Summary: Summary{Total: len(scenario.Entries), NotStarted: len(scenario.Entries)}}
	for _, entry := range scenario.Entries {
		if err := ctx.Err(); err != nil {
			result.Err, result.ErrorPosition = err, entry.Position
			return result
		}
		result.Summary.Started++
		result.Summary.NotStarted--
		response, err := r.executor.Execute(ctx, Request{URL: entry.URL})
		current := EntryResult{Entry: entry, Status: response.Status}
		if err != nil {
			current.Outcome, current.Status = Failed, 0
			result.Entries = append(result.Entries, current)
			result.Err, result.ErrorPosition = err, entry.Position
			return result
		}
		result.Summary.Completed++
		if entry.ExpectedStatus == nil {
			current.Outcome = Unvalidated
			result.Summary.WithoutValidation++
		} else if assertion.Status(*entry.ExpectedStatus, response.Status) {
			current.Outcome = Passed
			result.Summary.Validated++
		} else {
			current.Outcome = Mismatch
			result.Diverged = true
		}
		result.Entries = append(result.Entries, current)
		if result.Diverged {
			return result
		}
	}
	return result
}
