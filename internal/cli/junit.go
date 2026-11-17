package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"capimgrosso.com/httpgo/internal/report"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/suite"
)

func (c *Command) junitDiagnostic(run runner.Result, loadErr error) string {
	if loadErr != nil {
		return c.protection.sanitize(loadErr.Error()) + "\n"
	}
	var text bytes.Buffer
	local := *c
	local.err = &text
	// bytes.Buffer writes cannot fail. Reuse the text mode's safe diagnostics;
	// retaining only text avoids holding scenarios, payloads or sessions.
	_ = local.renderDiagnostic(run)
	return c.protection.sanitize(text.String())
}

func (c *Command) writeJUnit(path string, result suite.Result, diagnostics []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("criar diretório do relatório JUnit: %w", err)
	}
	open := c.openReport
	if open == nil {
		open = func(path string) (io.WriteCloser, error) {
			return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		}
	}
	writer, err := open(path)
	if err != nil {
		return fmt.Errorf("abrir relatório JUnit: %w", err)
	}
	// Apply final invocation protection to earlier text too: secrets discovered
	// in later files must not survive in the artifact. Always redact BEFORE XML
	// escaping, never chunk-by-chunk on the serialized stream.
	files := append([]suite.FileResult(nil), result.Files...)
	for i := range files {
		files[i].Name = c.protection.sanitize(files[i].Name)
	}
	result.Files = files
	safeDiagnostics := make([]string, len(diagnostics))
	for i, text := range diagnostics {
		safeDiagnostics[i] = c.protection.sanitize(text)
	}
	writeErr := report.WriteJUnit(writer, result, safeDiagnostics)
	return errors.Join(writeErr, writer.Close())
}
