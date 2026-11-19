package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"capimgrosso.com/httpgo/internal/report"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/suite"
)

func (c *Command) htmlEntries(run runner.Result) []report.HTMLEntry {
	entries := make([]report.HTMLEntry, 0, len(run.Entries))
	for i, entry := range run.Entries {
		var text bytes.Buffer
		local := *c
		local.out = &text
		// The same safe formatter as text mode, without retaining parser entries.
		_ = local.renderEntry(entry)
		url := displayURL(entry.DisplayURL)
		if entry.Outcome == runner.RenderFailed {
			url = "[URL não renderizada]"
		}
		entries = append(entries, report.HTMLEntry{
			Number: i + 1, Method: c.protection.sanitize(entry.Entry.Method),
			URL: c.protection.sanitize(url), Outcome: c.protection.sanitize(string(entry.Outcome)),
			Duration: entry.Duration, Diagnostic: c.protection.sanitize(text.String()),
		})
	}
	return entries
}

func (c *Command) writeHTML(path string, result suite.Result, details []report.HTMLFile) (string, error) {
	document := report.HTMLDocument{Summary: result.Summary}
	for i, file := range result.Files {
		current := report.HTMLFile{Result: file}
		if i < len(details) {
			current.Diagnostic, current.Entries = details[i].Diagnostic, details[i].Entries
		}
		// Secrets discovered in later files also protect earlier metadata. Redact
		// every string BEFORE html/template escapes it, never serialized chunks.
		current.Result.Name = c.protection.sanitize(current.Result.Name)
		current.Result.State = suite.State(c.protection.sanitize(string(current.Result.State)))
		current.Diagnostic = c.protection.sanitize(current.Diagnostic)
		for j := range current.Entries {
			entry := &current.Entries[j]
			entry.Method = c.protection.sanitize(entry.Method)
			entry.URL = c.protection.sanitize(entry.URL)
			entry.Outcome = c.protection.sanitize(entry.Outcome)
			entry.Diagnostic = c.protection.sanitize(entry.Diagnostic)
		}
		document.Files = append(document.Files, current)
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return "", fmt.Errorf("destino do relatório HTML é um diretório; informe um nome de arquivo")
	}
	if strings.HasSuffix(path, string(filepath.Separator)) {
		return "", fmt.Errorf("destino do relatório HTML exige um nome de arquivo")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", fmt.Errorf("criar diretório do relatório HTML: %w", err)
	}
	now := time.Now
	if c.reportNow != nil {
		now = c.reportNow
	}
	instant := now().UTC()
	stamp := fmt.Sprintf("%s%03dZ", instant.Format("20060102T150405"), instant.Nanosecond()/int(time.Millisecond))
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext) + "-" + stamp
	open := c.openReport
	if open == nil {
		open = func(path string) (io.WriteCloser, error) {
			return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		}
	}
	for suffix := 0; ; suffix++ {
		candidate := base
		if suffix != 0 {
			candidate += fmt.Sprintf("-%d", suffix)
		}
		candidate += ext
		writer, err := open(candidate)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("abrir relatório HTML: %w", err)
		}
		if err := errors.Join(report.WriteHTML(writer, document), writer.Close()); err != nil {
			return "", fmt.Errorf("gravar relatório HTML: %w", err)
		}
		return candidate, nil
	}
}
