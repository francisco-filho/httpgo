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

// GO-443: writers override success, mismatch and capture error without reprinting.
func TestCountCaptureWriterPriority(t *testing.T) {
	for _, mode := range []string{"capture success", "capture error", "comparison success", "comparison mismatch", "comparison error"} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/short=%v", mode, short), func(t *testing.T) {
				source := "GET http://example.test/first\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$\" count\n"
				if mode == "capture error" {
					source += "missing: jsonpath \"$.missing\"\n"
				}
				source += "GET http://example.test/after\nHTTP 200\n[Asserts]\njsonpath \"$\" count == {{baseline}}\nGET http://example.test/end"
				calls := 0
				cmd := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					body := `["private-order",null,null]`
					if calls == 2 {
						body = `[1,2,3]`
						if mode == "comparison mismatch" {
							body = `[1,2,3,4]`
						}
						if mode == "comparison error" {
							body = `invalid-private`
						}
					}
					return runner.Response{Status: 200, Body: []byte(body)}, nil
				}), BuildInfo{})
				successes, wantCalls := 0, 1
				if strings.HasPrefix(mode, "comparison") {
					successes, wantCalls = 1, 2
				}
				out := &failAfterWriter{successes: successes, cause: errors.New("private writer cause")}
				if short {
					out.cause = nil
				}
				var stderr bytes.Buffer
				cmd.SetArgs([]string{"test", writeScenario(t, source), "--render-asserts", "--secret", "baseline"})
				cmd.SetOut(out)
				cmd.SetErr(&stderr)
				if code := cmd.ExecuteContext(context.Background()); code != 2 || calls != wantCalls || out.calls != wantCalls || strings.Contains(out.String(), "resumo") || !strings.Contains(stderr.String(), "escrever o resultado") {
					t.Fatalf("code=%d calls=%d writes=%d out=%s err=%s", code, calls, out.calls, out.String(), stderr.String())
				}
				for _, bad := range []string{"private-order", "invalid-private", "private writer cause", "{{baseline}}"} {
					if strings.Contains(out.String()+stderr.String(), bad) {
						t.Fatalf("leaked %q", bad)
					}
				}
			})
		}
	}
}
