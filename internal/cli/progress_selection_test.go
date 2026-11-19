package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/suite"
)

func TestCompactSelectionAndColors(t *testing.T) {
	for _, stdoutTTY := range []bool{false, true} {
		for _, stderrTTY := range []bool{false, true} {
			for _, noColor := range []string{"absent", "", "1"} {
				t.Run(fmt.Sprintf("out=%t/err=%t/NO_COLOR=%s", stdoutTTY, stderrTTY, noColor), func(t *testing.T) {
					if noColor == "absent" {
						old, ok := os.LookupEnv("NO_COLOR")
						if err := os.Unsetenv("NO_COLOR"); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() {
							if ok {
								_ = os.Setenv("NO_COLOR", old)
							} else {
								_ = os.Unsetenv("NO_COLOR")
							}
						})
					} else {
						t.Setenv("NO_COLOR", noColor)
					}
					dir := t.TempDir()
					file := suiteFile(t, dir, "pass.httpgo", "GET http://example.test\nHTTP 200")
					var out, stderr bytes.Buffer
					command := NewRootCommand(&fakeExecutor{statuses: []int{200}}, BuildInfo{})
					command.isTerminal = func(w io.Writer) bool {
						if w == &out {
							return stdoutTTY
						}
						if w == &stderr {
							return stderrTTY
						}
						t.Fatal("detected wrapped writer")
						return false
					}
					command.SetOut(&out)
					command.SetErr(&stderr)
					command.SetArgs([]string{"test", file})
					if code := command.ExecuteContext(context.Background()); code != 0 {
						t.Fatalf("code=%d err=%q", code, stderr.String())
					}
					compact := stdoutTTY && stderrTTY
					if compact {
						if !strings.Contains(out.String(), "Testes executados: 1\n") || !strings.Contains(out.String(), "Sucesso/erros: 1/0\n") || strings.Contains(out.String(), "resumo") || !strings.Contains(stderr.String(), "pass.httpgo") {
							t.Fatalf("out=%q err=%q", out.String(), stderr.String())
						}
						colored := strings.Contains(stderr.String(), "\x1b[32mOK\x1b[0m")
						if colored != (noColor == "absent") {
							t.Fatalf("colors=%t err=%q", colored, stderr.String())
						}
					} else if !strings.Contains(out.String(), "resumo suíte:") || !strings.Contains(out.String(), "GET ") || stderr.Len() != 0 || strings.Contains(out.String(), "\x1b") {
						t.Fatalf("legacy changed: out=%q err=%q", out.String(), stderr.String())
					}
				})
			}
		}
	}
}

func TestCompactSummaryCountsAndDuration(t *testing.T) {
	for _, test := range []struct {
		name    string
		summary suite.Summary
		want    string
	}{
		{"mixed", suite.Summary{Processed: 3, Executed: 1, Passed: 1, Failed: 1, Errors: 1, Skipped: 2, Duration: 6 * time.Second}, "Testes executados: 3\nTempo: 6s\nTempo médio por teste: 2s\nSucesso/erros: 1/2\nNão executados: 2\n"},
		{"zero", suite.Summary{Skipped: 2, Duration: time.Second}, "Testes executados: 0\nTempo: 1s\nTempo médio por teste: não aplicável\nSucesso/erros: 0/0\nNão executados: 2\n"},
		{"pass", suite.Summary{Processed: 2, Passed: 2, Duration: 3 * time.Second}, "Testes executados: 2\nTempo: 3s\nTempo médio por teste: 1.5s\nSucesso/erros: 2/0\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			c := &Command{out: &out}
			want := "-----------------------------------------------\n" + test.want
			if err := c.renderCompactSuite(test.summary); err != nil || out.String() != want {
				t.Fatalf("out=%q want=%q err=%v", out.String(), want, err)
			}
		})
	}
}

func TestCompactDoesNotAffectReportsHelpOrPreexecutionErrors(t *testing.T) {
	for _, mode := range []string{"junit", "html", "help", "version", "usage", "discovery", "config"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			file := suiteFile(t, dir, "pass.httpgo", "GET http://example.test\nHTTP 200")
			args := []string{"test", file}
			wantCode := 0
			switch mode {
			case "junit", "html":
				args = append(args, "--output", mode, "--output-file", filepath.Join(dir, "report."+mode))
			case "help":
				args = []string{"--help"}
			case "version":
				args = []string{"--version"}
			case "usage":
				args = []string{"test", "--invalid"}
				wantCode = 2
			case "discovery":
				args = []string{"test", filepath.Join(dir, "missing")}
				wantCode = 2
			case "config":
				args = append(args, "--config", filepath.Join(dir, "missing.yml"))
				wantCode = 2
			}
			var out, stderr bytes.Buffer
			fake := &fakeExecutor{statuses: []int{200}}
			c := NewRootCommand(fake, BuildInfo{})
			c.isTerminal = func(io.Writer) bool { return true }
			c.SetOut(&out)
			c.SetErr(&stderr)
			c.SetArgs(args)
			if code := c.ExecuteContext(context.Background()); code != wantCode {
				t.Fatalf("code=%d err=%q", code, stderr.String())
			}
			if strings.Contains(out.String()+stderr.String(), "\x1b") || strings.Contains(out.String(), "Testes executados:") {
				t.Fatalf("compact leaked: out=%q err=%q", out.String(), stderr.String())
			}
		})
	}
}

func TestCompactSummaryWriterFailure(t *testing.T) {
	for _, short := range []bool{false, true} {
		t.Run(fmt.Sprint(short), func(t *testing.T) {
			file := suiteFile(t, t.TempDir(), "pass.httpgo", "GET http://example.test")
			writer := &suiteWriter{short: short}
			var stderr bytes.Buffer
			c := NewRootCommand(&fakeExecutor{statuses: []int{200}}, BuildInfo{})
			c.isTerminal = func(io.Writer) bool { return true }
			c.SetOut(writer)
			c.SetErr(&stderr)
			c.SetArgs([]string{"test", file})
			if code := c.ExecuteContext(context.Background()); code != 2 || writer.calls != 1 || !strings.Contains(stderr.String(), "resumo da suíte") {
				t.Fatalf("code=%d calls=%d err=%q", code, writer.calls, stderr.String())
			}
		})
	}
}

func TestTerminalWriterRejectsNonTerminals(t *testing.T) {
	if terminalWriter(&bytes.Buffer{}) || terminalWriter(io.Discard) {
		t.Fatal("buffer detected as terminal")
	}
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if terminalWriter(file) {
		t.Fatal("character device is not a TTY")
	}
}
