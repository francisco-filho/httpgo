package cli

import (
	"context"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestRegexAssertionCLI(t *testing.T) {
	for _, tc := range []struct {
		decl, body, diagnostic string
		code                   int
	}{
		{`regex "id=(.*)" == "ok"`, "id=ok", "", 0},
		{`regex "id=(.*)" contains "other"`, "id=private-response", "grupo presente", 1},
		{`regex "id=(.*)" exists`, "absent", "grupo ausente", 1},
		{`regex "id=(.*)" not contains "x"`, "absent", "", 0},
		{`regex "no-group" not contains "x"`, "no-group", "exige primeiro grupo", 2},
	} {
		t.Run(tc.decl, func(t *testing.T) {
			calls := 0
			fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(tc.body)}, nil
			})
			code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\n"+tc.decl+"\nGET http://example.test/next"))
			wantCalls := 1
			if tc.code == 0 {
				wantCalls = 2
			}
			if code != tc.code || calls != wantCalls || !strings.Contains(stderr, tc.diagnostic) || strings.Contains(out+stderr, "private-response") {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out, stderr)
			}
			if tc.code != 0 && !strings.Contains(stderr, ":4:") {
				t.Fatal(stderr)
			}
		})
	}
}
