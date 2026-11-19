package cli

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

func TestRegexCaptureWritersAndDerivedProtection(t *testing.T) {
	value := `private"\\雪{{missing}}`
	for _, mode := range []string{"success", "status", "mixed", "collision", "pending", "transport", "write", "write-status", "write-mixed", "short"} {
		t.Run(mode, func(t *testing.T) {
			source := "GET http://example.test/first\nHTTP 200\n[Captures]\nx: header \"Set-Cookie\" regex \"refresh=([^;]+)\"\n"
			if mode == "mixed" || mode == "write-mixed" {
				source += "bad: header \"Missing\"\n"
			}
			if mode == "collision" {
				source += "x.child: header \"X\"\n"
			}
			if mode == "pending" {
				source += "[Asserts]\nheader \"Other\" == \"pending-private-operand\"\n"
			}
			source += "POST http://example.test/" + url.PathEscape(value) + "\nX-Value: pre{{x}}post\n{\"x\":{{x}},\"text\":\"prefix{{x}}\"}\nHTTP 200\nGET http://example.test/last"
			var out, stderr bytes.Buffer
			calls := 0
			command := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
				calls++
				if calls == 1 {
					return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: "refresh=" + value + "; HttpOnly"}, {Name: "X", Value: "another"}}}, nil
				}
				if calls == 2 {
					if req.Headers[0].Value != "pre"+value+"post" {
						t.Fatal("operational value changed")
					}
					if mode == "transport" {
						return runner.Response{}, errors.New(url.QueryEscape(value))
					}
					if mode == "status" || mode == "write-status" {
						return runner.Response{Status: 503}, nil
					}
				}
				return runner.Response{Status: 200}, nil
			}), BuildInfo{})
			command.SetOut(&out)
			command.SetErr(&stderr)
			var failing *failAfterWriter
			if strings.HasPrefix(mode, "write") {
				failing = &failAfterWriter{cause: errors.New(url.PathEscape(value))}
				if mode == "write-status" {
					failing.successes = 1
				}
				command.SetOut(failing)
			}
			if mode == "short" {
				command.SetOut(shortWriter{})
			}
			command.SetArgs([]string{"test", writeScenario(t, source)})
			code := command.ExecuteContext(context.Background())
			wantCode, wantCalls := 0, 3
			switch mode {
			case "status":
				wantCode, wantCalls = 1, 2
			case "pending":
				wantCode, wantCalls = 1, 1
			case "mixed", "collision", "write", "write-mixed", "short":
				wantCode, wantCalls = 2, 1
			case "transport", "write-status":
				wantCode, wantCalls = 2, 2
			}
			if code != wantCode || calls != wantCalls {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out.String(), stderr.String())
			}
			combined := out.String() + stderr.String()
			if failing != nil {
				combined += failing.String()
				if failing.calls != failing.successes+1 || strings.Contains(failing.String(), "resumo") {
					t.Fatal("reused failing writer")
				}
			}
			for _, variant := range template.SecretVariants(value) {
				if strings.Contains(combined, variant) {
					t.Fatal("secret group leaked")
				}
			}
			if mode == "pending" && strings.Contains(combined, "pending-private-operand") {
				t.Fatal("pending operand leaked")
			}
		})
	}
}

func TestRegexSuiteProtectionWithoutResolutionSharing(t *testing.T) {
	const token = "fiction-regex-suite-token"
	dir := t.TempDir()
	suiteFile(t, dir, "01.hurl", "GET http://example.test/login\n[Captures]\ntoken: header \"Set-Cookie\" regex \"refresh=([^;]+)\"\nmissing: header \"Absent\"")
	suiteFile(t, dir, "02.hurl", "GET http://example.test/neutral")
	suiteFile(t, dir, "03-"+token+".hurl", "GET http://example.test/echo\nHTTP 200\n[Asserts]\nheader \"X-Echo\" == \"different\"")
	suiteFile(t, dir, "04.hurl", "GET http://example.test/never\nX-Value: {{token}}")
	calls := 0
	code, out, stderr := execute(t, context.Background(), executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		calls++
		if strings.HasSuffix(req.URL, "/login") {
			return runner.Response{Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: "refresh=" + token + "; HttpOnly"}}}, nil
		}
		if strings.HasSuffix(req.URL, "/echo") {
			return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "X-Echo", Value: token}}}, nil
		}
		return runner.Response{Status: 200}, nil
	}), "test", dir)
	if code != 2 || calls != 3 || strings.Contains(out+stderr, token) || !strings.Contains(out, "[oculto]") {
		t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out, stderr)
	}
	// A separate command invocation inherits neither protection nor resolution.
	code, out, stderr = execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		return runner.Response{Status: 200}, nil
	}), "test", writeScenario(t, "GET http://example.test/"+token))
	if code != 0 || !strings.Contains(out, token) {
		t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
	}
}
