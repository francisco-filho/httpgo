package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/suite"
	"golang.org/x/term"
)

func terminalWriter(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

// fileProgress retains only the current display line, never execution data.
// Events are synchronous: a pending request leaves its last update visible.
type fileProgress struct {
	command     *Command
	name        string
	total, done int
	known       bool
}

func (p *fileProgress) loading(name string) error {
	p.name = filepath.Base(filepath.FromSlash(name))
	p.total, p.done, p.known = 0, 0, false
	return p.draw(p.bar(), false)
}

func (p *fileProgress) loaded(_ string, total int) error {
	p.total, p.known = total, true
	return p.draw(p.bar(), false)
}

func (p *fileProgress) entry(runner.EntryResult) error {
	p.done++ // One terminal logical outcome, including preparation errors.
	return p.draw(p.bar(), false)
}

func (p *fileProgress) bar() string {
	filled := 0
	if p.known && p.total > 0 {
		filled = 10 * p.done / p.total
	}
	return "|" + strings.Repeat("=", filled) + ">" + strings.Repeat(" ", 10-filled)
}

func (p *fileProgress) draw(state string, final bool) error {
	total := "?"
	if p.known {
		total = fmt.Sprint(p.total)
	}
	ending := ""
	if final {
		ending = "\n"
	}
	name := p.name
	if p.command.color {
		name = "\x1b[1m" + name + "\x1b[0m"
		color := "\x1b[36m"
		switch state {
		case "OK":
			color = "\x1b[32m"
		case "ERRO":
			color = "\x1b[31m"
		}
		state = color + state + "\x1b[0m"
	}
	// The protected writer sanitizes the complete update before it reaches stderr.
	_, err := fmt.Fprintf(p.command.err, "\r\x1b[2K- %s  %s  %d/%s%s", name, state, p.done, total, ending)
	return err
}

func (p *fileProgress) finish(file suite.FileResult, run runner.Result, loadErr error) error {
	state := "ERRO"
	switch file.State {
	case suite.Passed:
		state = "OK"
	case suite.Skipped:
		p.name = filepath.Base(filepath.FromSlash(file.Name))
		p.done, p.total, p.known = 0, 0, false
		state = "NÃO EXECUTADO"
	case suite.Failed, suite.Errored:
		// Preserve suite classification; the progress bar never evaluates assertions.
	default:
		return fmt.Errorf("resultado de arquivo desconhecido")
	}
	if err := p.draw(state, true); err != nil {
		return err
	}
	return p.diagnostic(run, loadErr)
}
