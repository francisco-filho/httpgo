package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

// GO-360: write failure has priority 2 on success, mismatch and evaluation error,
// without retrying a failed writer or sending the dependent request.
func TestFilterWriterFailureOverridesOutcomes(t *testing.T) {
	for _, body := range []string{`[{"x":1,"v":true}]`, `[{"x":1,"v":false}]`, `[{"x":1,"v":true},null]`} {
		for _, short := range []bool{false, true} {
			calls := 0
			c := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(body)}, nil
			}), BuildInfo{})
			c.SetArgs([]string{"test", writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$[?(@.x==1)].v\" == true\nGET http://example.test/never")})
			var stderr bytes.Buffer
			c.SetErr(&stderr)
			writer := &failAfterWriter{cause: errors.New("write-private")}
			if short {
				c.SetOut(shortWriter{})
			} else {
				c.SetOut(writer)
			}
			code := c.ExecuteContext(context.Background())
			if code != 2 || calls != 1 || !strings.Contains(stderr.String(), "escrever o resultado") || strings.Contains(stderr.String(), "write-private") || !short && writer.calls != 1 {
				t.Fatalf("code=%d calls=%d err=%s", code, calls, stderr.String())
			}
		}
	}
}

// GO-355: sanitize literals and derived summaries before streams. A pending
// capture is not evaluated early just to obtain protection.
func TestFilterProtectedDiagnosticsCLI(t *testing.T) {
	for _, body := range []string{`[{"key":"known-private","v":"unrelated-private"},{"key":"known-private","v":"second"}]`, `[{"key":"known-private"},null]`, `not JSON`} {
		source := "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$[?(@.key=='known-\\\\u0070rivate')].v\" count == 0\n[Captures]\nfuture: regex \"(.*)\""
		code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			return runner.Response{Status: 200, Body: []byte(body)}, nil
		}), "test", writeScenario(t, source), "--secret", "future")
		if code == 0 || !strings.Contains(stderr, "jsonpath") && !strings.Contains(stderr, "consulta") || strings.Contains(out+stderr, "known-") || strings.Contains(out+stderr, "u0070") || strings.Contains(out+stderr, "unrelated-private") || strings.Contains(out+stderr, "contagem=") || strings.Contains(stderr, "erro de captura") {
			t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
		}
	}
}
