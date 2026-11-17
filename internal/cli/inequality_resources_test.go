package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

// GO-429: errors and short writes override success/mismatch/evaluation without reprinting.
func TestJSONInequalityWriterPriority(t *testing.T) {
	for _, mode := range []string{"success", "mismatch", "expected", "decode"} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/short=%v", mode, short), func(t *testing.T) {
				operand := `"prefix-{{old}}-suffix"`
				if mode == "expected" {
					operand = `{{missing}}`
				}
				file := writeScenario(t, "GET http://example.test/login\nHTTP 200\n[Captures]\nold: jsonpath \"$\"\nGET http://example.test/renew\nHTTP 200\n[Asserts]\njsonpath \"$\" != "+operand+"\nGET http://example.test/never")
				calls := 0
				cmd := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					body := `"old-private"`
					if calls == 2 {
						body = `"new-private"`
						if mode == "mismatch" {
							body = `"prefix-old-private-suffix"`
						}
						if mode == "decode" {
							body = `bad-private`
						}
					}
					return runner.Response{Status: 200, Body: []byte(body)}, nil
				}), BuildInfo{})
				out := &failAfterWriter{successes: 1, cause: errors.New("private writer cause")}
				if short {
					out.cause = nil
				}
				var stderr bytes.Buffer
				cmd.SetArgs([]string{"test", file, "--render-asserts", "--secret", "old"})
				cmd.SetOut(out)
				cmd.SetErr(&stderr)
				if code := cmd.ExecuteContext(context.Background()); code != 2 || calls != 2 || out.calls != 2 || strings.Contains(out.String(), "resumo") || !strings.Contains(stderr.String(), "escrever o resultado") {
					t.Fatalf("code=%d calls=%d writes=%d out=%s err=%s", code, calls, out.calls, out.String(), stderr.String())
				}
				for _, bad := range []string{"old-private", "new-private", "bad-private", "private writer cause", "{{old}}"} {
					if strings.Contains(out.String()+stderr.String(), bad) {
						t.Fatal("unsafe output")
					}
				}
			})
		}
	}
}
