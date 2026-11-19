package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

// GO-499: failed or short text writes override success, mismatch and query
// errors, never cause another HTTP attempt and do not echo writer causes.
func TestRegexAssertionWriterFailures(t *testing.T) {
	for _, body := range []string{"id=expected", "id=different", "\xff"} {
		for _, short := range []bool{false, true} {
			calls := 0
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(body)}, nil
			}), BuildInfo{})
			command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\nregex \"id=(.*)\" == \"expected\"\nGET http://example.test/never")})
			var stderr bytes.Buffer
			command.SetErr(&stderr)
			writer := &failAfterWriter{cause: errors.New("write-private")}
			if short {
				command.SetOut(shortWriter{})
			} else {
				command.SetOut(writer)
			}
			if code := command.ExecuteContext(context.Background()); code != 2 || calls != 1 || strings.Contains(stderr.String(), "private") || !strings.Contains(stderr.String(), "escrever o resultado") {
				t.Fatalf("code=%d calls=%d err=%s", code, calls, stderr.String())
			}
			if !short && writer.calls != 1 {
				t.Fatal(writer.calls)
			}
		}
	}
}

func TestRegexAssertionJUnitWriterFailures(t *testing.T) {
	for _, body := range []string{"id=expected", "id=different", "\xff"} {
		for _, mode := range []string{"write", "short", "close"} {
			t.Run(body+mode, func(t *testing.T) {
				calls := 0
				command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					return runner.Response{Status: 200, Body: []byte(body)}, nil
				}), BuildInfo{})
				source := "GET http://example.test\nHTTP 200\n[Asserts]\nregex \"id=(.*)\" == \"expected\""
				command.SetArgs([]string{"test", writeScenario(t, source), "--output", "junit", "--output-file", filepath.Join(t.TempDir(), "report.xml")})
				var out, stderr bytes.Buffer
				command.SetOut(&out)
				command.SetErr(&stderr)
				writer := &regexReportWriter{mode: mode}
				command.openReport = func(string) (io.WriteCloser, error) { return writer, nil }
				if code := command.ExecuteContext(context.Background()); code != 2 || calls != 1 || !writer.closed || out.Len() != 0 || strings.Contains(stderr.String(), "private") || strings.Contains(stderr.String(), "gerado") {
					t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out.String(), stderr.String())
				}
			})
		}
	}
}

type regexReportWriter struct {
	mode   string
	closed bool
}

func (w *regexReportWriter) Write(p []byte) (int, error) {
	if w.mode == "write" {
		return 0, errors.New("write-private")
	}
	if w.mode == "short" {
		return 0, nil
	}
	return len(p), nil
}
func (w *regexReportWriter) Close() error {
	w.closed = true
	if w.mode == "close" {
		return errors.New("close-private")
	}
	return nil
}
