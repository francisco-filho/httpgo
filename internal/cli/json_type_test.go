package cli

import (
	"context"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestJSONTypeDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		expression, body, diagnostic string
		code                         int
	}{
		{`jsonpath "$.x" isInteger`, `{"x":3.0}`, `notação não inteira`, 1},
		{`jsonpath "$.x" isString`, `{"x":{"private-value":null}}`, `recebido tipo objeto`, 1},
		{`jsonpath "$.x" isEmpty`, `{}`, `recebido ausente`, 1},
		{`jsonpath "$.x" isBoolean`, `{"x":false}`, ``, 0},
		{`jsonpath "$.x.y" isEmpty`, `{"x":null}`, `erro de avaliação JSON`, 2},
	} {
		calls := 0
		file := writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n  "+tc.expression+"\nGET http://example.test/next")
		code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			calls++
			return runner.Response{Status: 200, Body: []byte(tc.body)}, nil
		}), "test", file)
		wantCalls := 1
		if tc.code == 0 {
			wantCalls = 2
		}
		if code != tc.code || calls != wantCalls || !strings.Contains(stderr, tc.diagnostic) || strings.Contains(out+stderr, "private-value") || strings.Contains(stderr, "esperado") {
			t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
		}
		if tc.code != 0 && !strings.Contains(stderr, ":4:3:") {
			t.Fatal(stderr)
		}
	}
}
