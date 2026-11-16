package cli

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestMembershipCLI(t *testing.T) {
	for index, tc := range []struct {
		line, body, diagnostic string
		code                   int
	}{
		{`jsonpath "$.codes" contains "REQUIRED"`, `{"codes":["REQUIRED"]}`, "", 0},
		{`jsonpath "$" contains 2`, `[1,"private"]`, "contains: esperado 2, recebido tipo array", 1},
		{`jsonpath "$" not contains 1`, `[1,"private"]`, "not contains: esperado 1, recebido tipo array", 1},
		{`jsonpath "$.x" not exists`, `{"x":null}`, "not exists: esperado ausente, recebido presente", 1},
		{`jsonpath "$.x" not exists`, `{}`, "", 0},
		{`jsonpath "$" not contains null`, `null`, "pertencimento exige string ou array", 1},
		{`jsonpath "$" not contains 1`, `"private"`, "busca em string exige esperado string", 1},
		{`jsonpath "$.x.y" not exists`, `{"x":null}`, "erro de avaliação JSON", 2},
		{`jsonpath "$" not contains 1`, `invalid`, "erro de avaliação JSON", 2},
		{`header "Set-Cookie" not contains "Domain"`, `invalid`, "not contains", 1},
	} {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			calls := 0
			code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(tc.body), Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: "private; Domain=x"}}}, nil
			}), "test", writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n  "+tc.line+"\nGET http://example.test/next"))
			wantCalls := 1
			if tc.code == 0 {
				wantCalls = 2
			}
			if code != tc.code || calls != wantCalls || !strings.Contains(stderr, tc.diagnostic) || strings.Contains(out+stderr, "private") {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
			if tc.code != 0 && !strings.Contains(stderr, ":4:3:") {
				t.Fatal(stderr)
			}
		})
	}
}

func TestMembershipInvalidCLINeverSends(t *testing.T) {
	for _, line := range []string{`jsonpath "$" contains []`, `jsonpath "$" not contains {}`, `jsonpath "$" not exists null`, `header "X" not contains 1`, `jsonpath "$" not`, `jsonpath "$" count not contains 1`, `jsonpath "$" not matches "x"`, `header "X" not exists`} {
		f := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), f, "test", writeScenario(t, "GET http://example.test\nGET http://example.test/late\nHTTP *\n[Asserts]\n"+line))
		if code != 2 || len(f.calls) != 0 || !strings.Contains(stderr, ":5:1:") {
			t.Fatalf("code=%d out=%s err=%s executor=%+v", code, out, stderr, f)
		}
	}
}
