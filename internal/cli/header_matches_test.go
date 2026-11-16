package cli

import (
	"context"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestHeaderMatchesCLI(t *testing.T) {
	for _, tc := range []struct {
		expression string
		headers    []query.HeaderValue
		code       int
		diagnostic string
	}{
		{`header "x-test" matches "^first,whole$"`, []query.HeaderValue{{Name: "X-Test", Value: "first,whole"}, {Name: "X-Test", Value: "other"}}, 0, ""},
		{`header "X-Test" matches "^other$"`, []query.HeaderValue{{Name: "X-Test", Value: "first"}, {Name: "X-Test", Value: "other"}}, 1, `esperado "^other$", recebido "first"`},
		{`header "X-Test" matches ""`, nil, 1, "recebido ausente"},
		{`header "X-Test" matches "^x$"`, []query.HeaderValue{{Name: "X-Test", Value: ""}}, 1, `recebido ""`},
		{`header "X-Test" matches "^$"`, []query.HeaderValue{{Name: "X-Test", Value: ""}}, 0, ""},
		{`header "Set-Cookie" matches "expected-private"`, []query.HeaderValue{{Name: "Set-Cookie", Value: "actual-private"}}, 1, `esperado "[oculto]", recebido "[oculto]"`},
	} {
		calls := 0
		fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			calls++
			return runner.Response{Status: 200, Headers: tc.headers}, nil
		})
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\n"+tc.expression+"\nGET http://example.test/next"))
		wantCalls := 2
		if tc.code != 0 {
			wantCalls = 1
		}
		if code != tc.code || calls != wantCalls || !strings.Contains(stderr, tc.diagnostic) || strings.Contains(out+stderr, "private") {
			t.Fatalf("%s: code=%d calls=%d out=%q err=%q", tc.expression, code, calls, out, stderr)
		}
		if code == 1 && (!strings.Contains(stderr, ":4:1: assertion header") || !strings.Contains(stderr, "matches")) {
			t.Fatalf("err=%q", stderr)
		}
	}
}

func TestHeaderMatchesInvalidPreflightCLI(t *testing.T) {
	for _, pattern := range []string{`"private["`, `"(?=private)"`, `"(private)\\1"`, `"\x41private"`, `"\qprivate"`, `/private/`, `"private" "extra"`} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/late\nHTTP *\n[Asserts]\n  header \"Set-Cookie\" matches "+pattern))
		if code != 2 || len(fake.calls) != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":6:3:") || strings.Contains(stderr, "private") {
			t.Fatalf("pattern=%s code=%d out=%q err=%q calls=%d", pattern, code, out, stderr, len(fake.calls))
		}
	}
}
