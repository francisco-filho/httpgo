package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestHeaderHelpContract(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%q", code, stderr)
		}
		for _, want := range []string{"HTTP *", "[Asserts]", `header "Content-Type" contains "application/json"`, `header "X-Request-Id" exists`, `header "X-Request-Id" == "req-123"`, "primeira ocorrência", "ausente de", "não interpola", "headers, capturas", "Set-Cookie", "Diferenças Hurl 8.0.0"} {
			if !strings.Contains(out, want) {
				t.Fatalf("help lacks %q", want)
			}
		}
	}
}

func TestHeaderAssertionsCLI(t *testing.T) {
	for _, tc := range []struct {
		expression string
		headers    []query.HeaderValue
		code       int
		diagnostic string
	}{
		{`header "x-test" == "first,whole"`, []query.HeaderValue{{Name: "X-Test", Value: "first,whole"}, {Name: "x-test", Value: "other"}}, 0, ""},
		{`header "X-Test" == "other"`, []query.HeaderValue{{Name: "X-Test", Value: "first"}, {Name: "x-test", Value: "other"}}, 1, `esperado "other", recebido "first"`},
		{`header "X-Test" contains "irs"`, []query.HeaderValue{{Name: "X-Test", Value: "first"}}, 0, ""},
		{`header "X-Test" contains "FIRST"`, []query.HeaderValue{{Name: "X-Test", Value: "first"}}, 1, `recebido "first"`},
		{`header "X-Test" exists`, []query.HeaderValue{{Name: "X-Test", Value: ""}}, 0, ""},
		{`header "X-Test" exists`, nil, 1, "esperado presente, recebido ausente"},
		{`header "X-Test" == ""`, nil, 1, `esperado "", recebido ausente`},
		{`header "X-Test" == ""`, []query.HeaderValue{{Name: "X-Test", Value: ""}}, 0, ""},
		{`header "X-Test" == "nonempty"`, []query.HeaderValue{{Name: "X-Test", Value: ""}}, 1, `recebido ""`},
		{`header "X-Test" contains ""`, nil, 1, "recebido ausente"},
		{`header "X-Test" contains ""`, []query.HeaderValue{{Name: "X-Test", Value: ""}}, 0, ""},
		{`header "X-Test" == "{{name}}"`, []query.HeaderValue{{Name: "X-Test", Value: "{{name}}"}}, 0, ""},
	} {
		for _, expectation := range []string{"HTTP 200", "HTTP *"} {
			calls := 0
			fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Headers: tc.headers}, nil
			})
			source := "GET http://example.test/first\n" + expectation + "\n[Asserts]\n" + tc.expression + "\nGET http://example.test/next"
			code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, source), "--var", "name=resolved")
			wantCalls := 2
			if tc.code == 1 {
				wantCalls = 1
			}
			if code != tc.code || calls != wantCalls || !strings.Contains(stderr, tc.diagnostic) {
				t.Fatalf("%s %s: code=%d calls=%d out=%q err=%q", expectation, tc.expression, code, calls, out, stderr)
			}
			if code == 1 && (!strings.Contains(stderr, ":4:1: assertion header") || !strings.Contains(out, "não_iniciadas=1")) {
				t.Fatalf("out=%q err=%q", out, stderr)
			}
		}
	}
}

func TestHeaderSensitiveDiagnosticsCLI(t *testing.T) {
	for _, name := range []string{"Authorization", "pRoXy-Authorization", "Cookie", "sEt-CoOkIe", "X-Ordinary"} {
		source := fmt.Sprintf("GET http://example.test/actual-private\nHTTP *\n[Asserts]\nheader %q == \"expected-private\"\nGET http://example.test/last", name)
		fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			return runner.Response{Status: 404, Headers: []query.HeaderValue{{Name: name, Value: "actual-private"}, {Name: "Set-Cookie", Value: "actual-private"}}}, nil
		})
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, source), "--var", "token=expected-private", "--secret", "token")
		if code != 1 || strings.Contains(out+stderr, "actual-private") || strings.Contains(out+stderr, "expected-private") || !strings.Contains(stderr, "[oculto]") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}

func TestHeaderWriteFailuresTakePriority(t *testing.T) {
	for _, writer := range []io.Writer{shortWriter{}, &failAfterWriter{cause: errors.New("private")}} {
		calls := 0
		command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			calls++
			return runner.Response{Status: 200}, nil
		}), BuildInfo{})
		command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\nheader \"X\" exists\nGET http://example.test/next")})
		command.SetOut(writer)
		var stderr bytes.Buffer
		command.SetErr(&stderr)
		if code := command.ExecuteContext(context.Background()); code != 2 || calls != 1 || !strings.Contains(stderr.String(), "escrever o resultado") {
			t.Fatalf("code=%d calls=%d err=%q", code, calls, stderr.String())
		}
	}
}

func TestHeaderInvalidLateNeverSends(t *testing.T) {
	for _, suffix := range []string{"[Asserts]", "HTTP 200\n[Asserts]\nheader \"Authorization\" == private", "HTTP *\n[Asserts]\njsonpath \"$\" == \"private\""} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/late\n"+suffix))
		if code != 2 || out != "" || len(fake.calls) != 0 || strings.Contains(stderr, "private") || !strings.Contains(stderr, ":4:") && !strings.Contains(stderr, ":6:") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}
