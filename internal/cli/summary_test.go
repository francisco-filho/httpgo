package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/suite"
)

func TestTextSummariesCountAllTerminalStepsAcrossModes(t *testing.T) {
	for _, mode := range []string{"legacy", "compact", "verbose", "bodies"} {
		for _, fast := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fast=%t", mode, fast), func(t *testing.T) {
				dir := t.TempDir()
				for i, source := range []string{
					"GET http://example.test/ok\nGET http://example.test/ok\nHTTP *\nGET http://example.test/ok\nHTTP 200",
					"GET http://example.test/status\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200\nGET http://example.test/never",
					"GET http://example.test/{{missing}}",
					"GET http://example.test/capture\nHTTP 200\n[Captures]\nx: jsonpath \"$.absent\"",
					"invalid syntax without a step",
					"GET http://example.test/transport",
					"GET http://example.test/assertion\nHTTP 200\n[Asserts]\njsonpath \"$.ok\" == true",
					"POST http://example.test/file\nfile,missing.json;\nHTTP 200",
				} {
					suiteFile(t, dir, fmt.Sprintf("%02d.httpgo", i), source)
				}
				var out, stderr bytes.Buffer
				calls := 0
				c := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
					calls++
					if strings.HasSuffix(req.URL, "/never") || strings.HasSuffix(req.URL, "/file") {
						t.Fatal("unexpected HTTP", req.URL)
					}
					if strings.HasSuffix(req.URL, "/transport") {
						return runner.Response{}, errors.New("raw-private-cause")
					}
					status := 200
					if strings.HasSuffix(req.URL, "/status") {
						status = 503
					}
					return runner.Response{Status: status, Body: []byte(`{"ok":false}`)}, nil
				}), BuildInfo{})
				c.isTerminal = func(io.Writer) bool { return mode == "compact" }
				c.SetOut(&out)
				c.SetErr(&stderr)
				args := []string{"test", dir}
				if mode == "verbose" {
					args = append(args, "-v")
				}
				if mode == "bodies" {
					args = append(args, "-vv")
				}
				if fast {
					args = append(args, "--fail-fast")
				}
				c.SetArgs(args)
				code := c.ExecuteContext(context.Background())
				wantCode, wantCalls := 2, 8
				count, steps, scenarioStates, testStates := "Cenários executados: 8\n", "Testes executados: 9\n", "Sucesso/erros: 1/7\n", "Sucesso/erros: 3/6\n"
				if fast {
					wantCode, wantCalls = 1, 5
					count, steps, scenarioStates, testStates = "Cenários executados: 2\n", "Testes executados: 4\n", "Sucesso/erros: 1/1\nNão executados: 6\n", "Sucesso/erros: 3/1\n"
				}
				if code != wantCode || calls != wantCalls {
					t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out.String(), stderr.String())
				}
				summary := out.String()
				if !strings.Contains(summary, count) || !strings.Contains(summary, steps) || !strings.Contains(summary, scenarioStates) || !strings.Contains(summary, testStates) || strings.Contains(summary, "resumo suíte:") {
					t.Fatalf("wrong counts: %q", summary)
				}
				if strings.Index(summary, "Tempo médio por Cenário:") > strings.Index(summary, steps) || strings.Index(summary, "Tempo médio por Teste:") > strings.LastIndex(summary, "Tempo:") {
					t.Fatalf("wrong order: %q", summary)
				}
				details := stderr.String()
				if mode == "legacy" {
					details = out.String()
				}
				if mode == "verbose" || mode == "bodies" {
					if !strings.Contains(details, "Sucesso: 3\n-----------------------------------------------\n") || !strings.Contains(details, "Sucesso: 0 - Erro: 1\n-----------------------------------------------\n") || strings.Contains(details, "Erro: 0") {
						t.Fatalf("wrong file summary: %q", details)
					}
				} else if strings.Contains(out.String()+stderr.String(), "Sucesso: ") || strings.Contains(out.String()+stderr.String(), "-----------------------------------------------") {
					t.Fatalf("verbose-only file summary in %s: out=%q err=%q", mode, out.String(), stderr.String())
				}
				if strings.Contains(out.String()+stderr.String(), "raw-private-cause") {
					t.Fatal("raw error leaked")
				}
			})
		}
	}
}

func TestScenarioSummaryHasNoInventedOrSkippedSteps(t *testing.T) {
	for _, tc := range []struct {
		file suite.FileResult
		want string
	}{
		{suite.FileResult{State: suite.Passed, Successful: 3}, "Sucesso: 3\n-----------------------------------------------\n"},
		{suite.FileResult{State: suite.Errored, Successful: 2, AssertionFailures: 1, RequestErrors: 1}, "Sucesso: 2 - Erro: 2\n-----------------------------------------------\n"},
		{suite.FileResult{State: suite.Errored}, "Sucesso: 0\n-----------------------------------------------\n"},
		{suite.FileResult{State: suite.Skipped}, ""},
	} {
		if got := scenarioSummaryText(tc.file); got != tc.want {
			t.Fatalf("file=%+v got=%q want=%q", tc.file, got, tc.want)
		}
	}
}

func TestVerboseScenarioSummaryWriteFailureStopsNextFile(t *testing.T) {
	for _, short := range []bool{false, true} {
		dir := t.TempDir()
		suiteFile(t, dir, "01.httpgo", "GET http://example.test/ok")
		suiteFile(t, dir, "02.httpgo", "GET http://example.test/never")
		writer := &suiteWriter{allowed: 2, short: short}
		calls := 0
		c := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			calls++
			return runner.Response{Status: 200}, nil
		}), BuildInfo{})
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetErr(writer)
		c.SetArgs([]string{"test", dir, "-v"})
		if code := c.ExecuteContext(context.Background()); code != 2 || calls != 1 || writer.calls != 3 || out.Len() != 0 {
			t.Fatalf("short=%t code=%d sends=%d writes=%d out=%q", short, code, calls, writer.calls, out.String())
		}
	}
}
