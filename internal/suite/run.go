package suite

import (
	"context"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

// State describes the entire file, not merely a successful status assertion.
type State string

const (
	Passed  State = "sucesso"
	Failed  State = "falha de assertion"
	Errored State = "erro"
	Skipped State = "não executado"
)

// FileResult retains counts only: no scenarios, payloads, sessions or responses.
// Known is false when the file was not successfully parsed, including skipped files.
type FileResult struct {
	Attempts                                     runner.AttemptSummary
	Name                                         string
	State                                        State
	Duration                                     time.Duration
	Known                                        bool
	Requests                                     runner.Summary
	Successful, AssertionFailures, RequestErrors int
}

type Summary struct {
	Attempts                                                    runner.AttemptSummary
	Files, Processed, Executed, Passed, Failed, Errors, Skipped int
	Requests                                                    runner.Summary // Totals cover only successfully parsed files.
	UnknownFiles                                                int
	Successful, AssertionFailures, RequestErrors                int
	Duration                                                    time.Duration
}

type Result struct {
	Files          []FileResult
	Summary        Summary
	Code           int
	Err, ReportErr error
	ReportPosition parser.Position
}

type Options struct {
	Request  runner.Options
	FailFast bool
}

// Run reports each file synchronously, before opening or sending the next one.
// The runner result and load error are transient diagnostic inputs to the CLI;
// only compact FileResults are retained. Presentation errors are always global.
func (s *Suite) Run(ctx context.Context, files []File, options Options, report func(FileResult, runner.Result, error) error) Result {
	started := s.now()
	result := Result{Summary: Summary{Files: len(files)}}
	stopped := false
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			result.Err = err
			result.Code = 2
			stopped = true
		}
		current := FileResult{Name: file.Name, State: Skipped}
		var execution runner.Result
		var loadErr error
		if !stopped {
			begin := s.now()
			execution, loadErr = s.RunFile(ctx, file, options.Request)
			current.Duration = max(time.Duration(0), s.now().Sub(begin))
			current.Known = loadErr == nil
			current.Requests = execution.Summary
			current.Attempts = execution.Attempts
			current.State = classify(execution, loadErr)
			for _, entry := range execution.Entries {
				switch entry.Outcome {
				case runner.Passed, runner.Unvalidated, runner.Wildcard:
					current.Successful++
				case runner.Mismatch, runner.HeaderMismatch, runner.JSONMismatch, runner.BodyMismatch:
					current.AssertionFailures++
				default:
					current.RequestErrors++
				}
			}
			switch current.State {
			case Failed:
				result.Code = max(result.Code, 1)
			case Errored:
				result.Code = 2
			}
			if options.FailFast && current.State != Passed {
				stopped = true
			}
			if err := ctx.Err(); err != nil {
				result.Err = err
				result.Code = 2
				stopped = true
			}
		}
		current.Name = s.protect(current.Name)
		result.Files = append(result.Files, current)
		result.Summary.add(current)
		if execution.ReportErr != nil {
			result.ReportPosition = execution.ErrorPosition
			result.ReportErr = execution.ReportErr
			result.Code = 2
			stopped = true
		}
		if result.ReportErr != nil {
			continue
		} // Do not retry presentation after a failed callback.
		if report != nil {
			if err := report(current, execution, loadErr); err != nil {
				result.ReportPosition = parser.Position{File: current.Name, Line: 1, Column: 1}
				result.ReportErr = err
				result.Code = 2
				stopped = true
			}
		}
	}
	if err := ctx.Err(); err != nil {
		result.Err = err
		result.Code = 2
	}
	// Later discoveries may protect a name seen before the value was known.
	for i := range result.Files {
		result.Files[i].Name = s.protect(result.Files[i].Name)
	}
	result.ReportPosition.File = s.protect(result.ReportPosition.File)
	result.Summary.Duration = max(time.Duration(0), s.now().Sub(started))
	return result
}

func classify(result runner.Result, loadErr error) State {
	if loadErr != nil || result.Err != nil || result.ReportErr != nil {
		return Errored
	}
	if result.Diverged {
		return Failed
	}
	return Passed
}

func (s *Summary) add(file FileResult) {
	if !file.Known {
		s.UnknownFiles++
	}
	switch file.State {
	case Passed:
		s.Passed++
	case Failed:
		s.Failed++
	case Errored:
		s.Errors++
	case Skipped:
		s.Skipped++
	}
	if file.State != Skipped {
		s.Processed++
	}
	if file.Requests.Started > 0 {
		s.Executed++
	}
	s.Attempts.Started += file.Attempts.Started
	s.Attempts.Completed += file.Attempts.Completed
	s.Attempts.StatusChecks += file.Attempts.StatusChecks
	s.Requests.Total += file.Requests.Total
	s.Requests.Started += file.Requests.Started
	s.Requests.Completed += file.Requests.Completed
	s.Requests.Validated += file.Requests.Validated
	s.Requests.WithoutValidation += file.Requests.WithoutValidation
	s.Requests.NotStarted += file.Requests.NotStarted
	s.Successful += file.Successful
	s.AssertionFailures += file.AssertionFailures
	s.RequestErrors += file.RequestErrors
}
