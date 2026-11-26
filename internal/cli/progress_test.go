package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestCompactProgressLifecycle(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	for _, mode := range []string{"pass", "mismatch", "pre-send", "parse", "retry", "cancel", "read"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			first := "GET http://example.test/one\nHTTP 200\nGET http://example.test/two\nHTTP *"
			switch mode {
			case "pre-send":
				first = "GET http://example.test/{{missing}}\nGET http://example.test/blocked"
			case "parse":
				first += "\n[Unsupported]"
			case "retry":
				first = "GET http://example.test/one\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200"
			}
			suiteFile(t, dir, "a/same.httpgo", first)
			second := suiteFile(t, dir, "b/same.httpgo", "GET http://example.test/last")
			var out, stderr bytes.Buffer
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			command := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
				calls++
				text := stderr.String()
				if calls == 1 && (!strings.Contains(text, "same.httpgo") || !strings.Contains(text, "0/")) {
					t.Fatalf("not visible before send: %q", text)
				}
				if strings.HasSuffix(req.URL, "/last") && mode == "mismatch" && !strings.Contains(text, "step 1 - Linha") {
					t.Fatalf("diagnostic not before next send: %q", text)
				}
				if mode == "read" && calls == 1 {
					if err := os.Remove(second); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "cancel" {
					cancel()
					return runner.Response{}, ctx.Err()
				}
				if mode == "mismatch" || (mode == "retry" && calls == 1) {
					return runner.Response{Status: 503}, nil
				}
				if mode == "retry" && calls == 2 && strings.Contains(text, "1/1") {
					t.Fatalf("retry advanced: %q", text)
				}
				return runner.Response{Status: 200}, nil
			}), BuildInfo{})
			command.isTerminal = func(io.Writer) bool { return true }
			command.SetOut(&out)
			command.SetErr(&stderr)
			args := []string{"test", dir}
			if mode == "pre-send" || mode == "parse" {
				args = append(args, "--fail-fast")
			}
			command.SetArgs(args)
			code := command.ExecuteContext(ctx)
			want := 0
			switch mode {
			case "mismatch":
				want = 1
			case "pre-send", "parse", "cancel", "read":
				want = 2
			}
			if code != want {
				t.Fatalf("code=%d want=%d out=%q err=%q", code, want, out.String(), stderr.String())
			}
			text := stderr.String()
			if strings.Count(text, "- same.httpgo") < 2 || strings.Contains(text, "- a/") || strings.Contains(text, "- b/") || strings.Contains(out.String(), "GET ") || strings.Contains(text, "status validado") {
				t.Fatalf("detailed success/path leaked: out=%q err=%q", out.String(), text)
			}
			for _, fragment := range map[string][]string{
				"pass": {"OK  2/2", "OK  1/1"}, "mismatch": {"ERRO  1/2", "step 1 - Linha", "[status]", "expected 200 => received 503"},
				"pre-send": {"ERRO  1/2", "step 1 - Linha", "[renderização]", "NÃO EXECUTADO  0/?"}, "parse": {"ERRO  0/?", "NÃO EXECUTADO  0/?"},
				"retry": {"OK  1/1"}, "cancel": {"ERRO  1/2", "NÃO EXECUTADO  0/?"}, "read": {"ERRO  0/?"},
			}[mode] {
				if !strings.Contains(text, fragment) {
					t.Fatalf("missing %q in %q", fragment, text)
				}
			}
			if mode == "parse" && strings.Contains(text, "step ") {
				t.Fatalf("invented step: %q", text)
			}
		})
	}
}

func TestCompactProgressWriterFailures(t *testing.T) {
	// Loading, total, step, final line, diagnostic: fail at each boundary.
	for _, short := range []bool{false, true} {
		for allowed := 0; allowed < 5; allowed++ {
			t.Run(fmt.Sprintf("short=%t/allowed=%d", short, allowed), func(t *testing.T) {
				dir := t.TempDir()
				suiteFile(t, dir, "01.httpgo", "GET http://example.test/one\nHTTP 200")
				suiteFile(t, dir, "02.httpgo", "GET http://example.test/blocked")
				calls := 0
				command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					return runner.Response{Status: 503}, nil
				}), BuildInfo{})
				command.isTerminal = func(io.Writer) bool { return true }
				writer := &suiteWriter{allowed: allowed, short: short}
				var out bytes.Buffer
				command.SetOut(&out)
				command.SetErr(writer)
				command.SetArgs([]string{"test", dir})
				code := command.ExecuteContext(context.Background())
				wantCalls := 0
				if allowed >= 2 {
					wantCalls = 1
				}
				if code != 2 || calls != wantCalls || writer.calls != allowed+1 || out.Len() != 0 {
					t.Fatalf("code=%d sends=%d writes=%d out=%q", code, calls, writer.calls, out.String())
				}
			})
		}
	}
}

func TestCompactRenewalProtectionSurvivesFileChange(t *testing.T) {
	t.Setenv("HTTPGO_PROGRESS_TOKEN", "old-private-progress")
	config := writeConfig(t, "version: 1\nvariables:\n  token: ${HTTPGO_PROGRESS_TOKEN}\n")
	dir := t.TempDir()
	suiteFile(t, dir, "01.httpgo", "GET http://example.test/old-private-progress\n[Captures]\ntoken: jsonpath \"$.token\"\nGET http://example.test/{{token}}\n[Captures]\ntoken: jsonpath \"$.token\"")
	suiteFile(t, dir, "02-old-private-progress-new-private-progress-renewed-private-progress.httpgo", "GET http://example.test/{{token}}\nHTTP 200\n[Asserts]\nheader \"X\" == \"prefix-{{token}}-suffix\"")
	var requests []string
	command := NewRootCommand(executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
		requests = append(requests, request.URL)
		token := "new-private-progress"
		if len(requests) == 2 {
			token = "renewed-private-progress"
		}
		return runner.Response{Status: 200, Body: []byte(`{"token":"` + token + `"}`)}, nil
	}), BuildInfo{})
	command.isTerminal = func(io.Writer) bool { return true }
	command.SetArgs([]string{"test", dir, "--config", config, "--render-asserts"})
	var out, stderr bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&stderr)
	if code := command.ExecuteContext(context.Background()); code != 1 || len(requests) != 3 || !strings.HasSuffix(requests[1], "/new-private-progress") || !strings.HasSuffix(requests[2], "/old-private-progress") {
		t.Fatalf("code=%d requests=%v stderr=%q", code, requests, stderr.String())
	}
	for _, secret := range []string{"old-private-progress", "new-private-progress", "renewed-private-progress", "prefix-old-private-progress-suffix"} {
		if strings.Contains(out.String()+stderr.String(), secret) {
			t.Fatalf("secret %q in output: %q %q", secret, out.String(), stderr.String())
		}
	}
}

func TestCompactFourthStepAndProtection(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	const secret = "fiction-progress-token"
	dir := t.TempDir()
	suiteFile(t, dir, "01.httpgo", "GET http://example.test/login\n[Captures]\ntoken: jsonpath \"$.token\"\nGET http://example.test/2\nGET http://example.test/3\nGET http://example.test/4\nHTTP 200\nGET http://example.test/blocked")
	suiteFile(t, dir, "02-"+secret+".httpgo", "GET http://example.test/last\nHTTP 200\n[Asserts]\nbody contains \""+secret+"\"")
	var out, stderr bytes.Buffer
	calls := 0
	command := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		calls++
		if strings.HasSuffix(req.URL, "/login") {
			return runner.Response{Status: 200, Body: []byte(`{"token":"` + secret + `"}`)}, nil
		}
		if strings.HasSuffix(req.URL, "/4") {
			return runner.Response{Status: 503}, nil
		}
		return runner.Response{Status: 200}, nil
	}), BuildInfo{})
	command.isTerminal = func(io.Writer) bool { return true }
	command.SetArgs([]string{"test", dir, "--secret", "token"})
	command.SetOut(&out)
	command.SetErr(&stderr)
	if code := command.ExecuteContext(context.Background()); code != 1 || calls != 5 {
		t.Fatalf("code=%d calls=%d err=%q", code, calls, stderr.String())
	}
	text := stderr.String()
	if !strings.Contains(text, "ERRO  4/5\n  GET http://example.test/4 -> step 4 - Linha") || !strings.Contains(text, "[oculto]") || strings.Contains(text+out.String(), secret) {
		t.Fatalf("out=%q err=%q", out.String(), text)
	}
}
