package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

func TestHeaderCaptureCLI(t *testing.T) {
	for _, mode := range []string{"success", "missing", "status", "pending assertion", "later transport", "explicit"} {
		t.Run(mode, func(t *testing.T) {
			name := "Set-Cookie"
			if mode == "explicit" {
				name = "X-Private"
			}
			value := `private"\\/雪{{missing}}`
			source := "GET http://example.test/catalog\nHTTP 200\n[Captures]\nx: header \"" + name + "\"\n"
			if mode == "missing" {
				source += "bad: header \"Missing\"\n"
			}
			if mode == "pending assertion" {
				source += "[Asserts]\nheader \"Other\" == \"pending-private\"\n"
			}
			source += "GET http://example.test/next\nX-Value: prefix{{x}}suffix\nHTTP 200\nGET http://example.test/last"
			calls := 0
			fake := executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
				calls++
				if calls == 1 {
					return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: name, Value: value}}}, nil
				}
				if calls == 2 {
					if req.Headers[0].Value != "prefix"+value+"suffix" {
						t.Fatal("changed operational header")
					}
					if mode == "later transport" {
						return runner.Response{}, errors.New(value)
					}
					if mode == "status" {
						return runner.Response{Status: 503}, nil
					}
				}
				return runner.Response{Status: 200}, nil
			})
			args := []string{"test", writeScenario(t, source)}
			if mode == "explicit" {
				args = append(args, "--secret", "x")
			}
			code, out, stderr := execute(t, context.Background(), fake, args...)
			wantCode, wantCalls := 0, 3
			switch mode {
			case "missing":
				wantCode, wantCalls = 2, 1
			case "pending assertion":
				wantCode, wantCalls = 1, 1
			case "later transport":
				wantCode, wantCalls = 2, 2
			case "status":
				wantCode, wantCalls = 1, 2
			}
			if code != wantCode || calls != wantCalls {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out, stderr)
			}
			for _, variant := range template.SecretVariants(value) {
				if strings.Contains(out+stderr, variant) {
					t.Fatal("secret leaked in streams")
				}
			}
			if mode == "pending assertion" && strings.Contains(out+stderr, "pending-private") {
				t.Fatal("pending operand leaked")
			}
		})
	}
}

func TestHeaderCaptureLateSyntaxPreventsAllRequests(t *testing.T) {
	calls := 0
	fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		return runner.Response{Status: 200}, nil
	})
	code, _, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/first\nGET http://example.test/late\n[Captures]\nx: header ETag"))
	if code != 2 || calls != 0 || !strings.Contains(stderr, ":4:1:") {
		t.Fatalf("code=%d calls=%d err=%q", code, calls, stderr)
	}
}
