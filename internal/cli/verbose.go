package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/suite"
)

// verboseProgress has no live line or response state. Each callback writes one
// definitive outcome, after resources and all applicable validations are done.
type verboseProgress struct {
	command *Command
	done    int
}

func (p *verboseProgress) loading(name string) error {
	p.done = 0
	_, err := fmt.Fprintln(p.command.err, filepath.Base(filepath.FromSlash(name)))
	return err
}

func (p *verboseProgress) entry(entry runner.EntryResult) error {
	p.done++
	failed := entry.Diagnostic != nil
	switch entry.Outcome {
	case runner.Passed, runner.Unvalidated, runner.Wildcard:
	default:
		failed = true
	}
	state := "[OK]"
	if failed {
		state = "[ERRO]"
	}
	if p.command.color {
		color := "\x1b[32m"
		if failed {
			color = "\x1b[31m"
		}
		state = color + state + "\x1b[0m"
	}
	url := displayURL(entry.DisplayURL)
	if entry.Outcome == runner.RenderFailed || entry.DisplayURL == "" {
		url = "[URL não renderizada]"
	}
	if _, err := fmt.Fprintf(p.command.err, "%s %s %s (%dms)\n", state, entry.Entry.Method, url, entry.Duration.Milliseconds()); err != nil {
		return err
	}
	if !failed {
		return nil
	}
	var diagnostic progressDiagnostic
	if d := entry.Diagnostic; d != nil {
		diagnostic = progressDiagnostic{position: d.Position, phase: d.Phase, message: d.Detail}
		if d.Query != "" {
			diagnostic.expression = jsonProgressExpression(d.Query, "", d.Count)
		}
		if d.Phase == "assertion" {
			if expression := assertionProgressExpression(entry, d.Position); expression != "" {
				diagnostic.expression = expression
			}
			if entry.BodyError != nil {
				diagnostic.expression = "body " + string(entry.BodyError.Operator)
			}
			if entry.RegexError != nil {
				diagnostic.expression = "regex " + string(entry.RegexError.Operator)
			}
		}
	} else {
		diagnostic, _ = p.command.progressDiagnostic(runner.Result{Entries: []runner.EntryResult{entry}, Diverged: true}, nil)
	}
	if err := p.command.writeProgressDiagnostic(diagnostic, p.done); err != nil {
		return err
	}
	if entry.Bodies != nil {
		if err := p.body("request", entry.Bodies.Request); err != nil {
			return err
		}
		return p.body("response", entry.Bodies.Response)
	}
	return nil
}

func (p *verboseProgress) body(name string, preview runner.BodyPreview) error {
	text := string(preview.State)
	if preview.State == runner.BodyJSON {
		text = strings.ReplaceAll(preview.Text, "\n", "\n  ")
		if preview.Truncated {
			text += "\n  [truncado: limite de 16384 bytes]"
		}
	}
	_, err := fmt.Fprintf(p.command.err, "  [%s body]\n  %s\n", name, text)
	return err
}

func (p *verboseProgress) finish(file suite.FileResult, run runner.Result, loadErr error) error {
	if file.State == suite.Skipped {
		_, err := fmt.Fprintf(p.command.err, "%s NÃO EXECUTADO\n", filepath.Base(filepath.FromSlash(file.Name)))
		return err
	}
	// Step diagnostics were already emitted before this callback. Load failures
	// have no invented step, and cancellation before a step may have no callback.
	if loadErr != nil || (p.done == 0 && run.Err != nil) {
		diagnostic, ok := p.command.progressDiagnostic(run, loadErr)
		if ok {
			return p.command.writeProgressDiagnostic(diagnostic, 0)
		}
	}
	return nil
}
