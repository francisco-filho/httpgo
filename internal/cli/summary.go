package cli

import (
	"fmt"
	"io"
	"time"

	"capimgrosso.com/httpgo/internal/suite"
)

// scenarioSummaryText counts terminal logical steps, not HTTP attempts or
// successful status checks. Load failures do not invent a failed step.
func scenarioSummaryText(file suite.FileResult) string {
	if file.State == suite.Skipped {
		return ""
	}
	text := fmt.Sprintf("Sucesso: %d", file.Successful)
	if failures := file.AssertionFailures + file.RequestErrors; failures > 0 {
		text += fmt.Sprintf(" - Erro: %d", failures)
	}
	return text + "\n-----------------------------------------------\n"
}

// renderSuite is shared by all text modes. Structured reports keep their own
// contracts and never use these presentation labels or denominators.
func (c *Command) renderSuite(s suite.Summary) error {
	failures := s.AssertionFailures + s.RequestErrors
	executed := s.Successful + failures
	average := func(count int) string {
		if count == 0 {
			return "não aplicável"
		}
		return (s.Duration / time.Duration(count)).String()
	}
	text := fmt.Sprintf("Cenários executados: %d\nTempo médio por Cenário: %s\nSucesso/erros: %d/%d\n", s.Processed, average(s.Processed), s.Passed, s.Failed+s.Errors)
	if s.Skipped > 0 {
		text += fmt.Sprintf("Não executados: %d\n", s.Skipped)
	}
	text += fmt.Sprintf("\nTestes executados: %d\nTempo médio por Teste: %s\nSucesso/erros: %d/%d\n\nTempo: %s\n", executed, average(executed), s.Successful, failures, s.Duration)
	_, err := io.WriteString(c.out, text)
	return err
}
