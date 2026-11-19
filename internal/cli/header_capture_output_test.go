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

func TestHeaderCaptureWritersAndDerivedProtection(t *testing.T) {
	value := `private"\\/雪{{missing}}`
	for _, mode := range []string{"success", "status", "missing", "render", "transport", "write", "write-status", "write-missing", "short"} {
		t.Run(mode, func(t *testing.T) {
			source := "GET http://example.test/first\nHTTP 200\n[Captures]\nx: header \"sEt-CoOkIe\"\n"
			if mode == "missing" || mode == "write-missing" {
				source += "bad: header \"Missing\"\n"
			}
			source += "POST http://example.test/" + url.PathEscape(value) + "\nX-Value: pre{{x}}post\n{\"x\":{{x}},\"text\":\"prefix{{x}}\"}\nHTTP 200\n"
			if mode == "render" {
				source += "GET http://example.test/{{absent}}\n"
			}
			source += "GET http://example.test/last"
			var out, stderr bytes.Buffer
			calls := 0
			command := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
				calls++
				if calls == 1 {
					return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: value}}}, nil
				}
				if calls == 2 {
					if req.Headers[0].Value != "pre"+value+"post" || !strings.Contains(string(req.Body), "prefix") {
						t.Fatal("operational value changed")
					}
					if mode == "status" || mode == "write-status" {
						return runner.Response{Status: 503}, nil
					}
					if mode == "transport" {
						return runner.Response{}, errors.New(url.QueryEscape(value))
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
			case "missing", "write", "write-missing", "short":
				wantCode, wantCalls = 2, 1
			case "render", "transport", "write-status":
				wantCode, wantCalls = 2, 2
			}
			if code != wantCode || calls != wantCalls {
				t.Fatalf("mode=%s code=%d calls=%d out=%q err=%q", mode, code, calls, out.String(), stderr.String())
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
					t.Fatal("secret leaked")
				}
			}
		})
	}
}
