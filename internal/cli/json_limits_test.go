package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestJSONLimitsWriterFailures(t *testing.T) {
	for _, line := range []string{`jsonpath "$" > 1`, `jsonpath "$" exists`, `jsonpath "$" count == 2`} {
		for _, body := range []string{`2`, `0`, `[]`, `invalid`} {
			for _, mode := range []string{"result error", "result short", "diagnostic error", "diagnostic short"} {
				t.Run(line+body+mode, func(t *testing.T) {
					calls := 0
					command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
						calls++
						return runner.Response{Status: 200, Body: []byte(body)}, nil
					}), BuildInfo{})
					var out, stderr bytes.Buffer
					var output, diagnostic io.Writer = &out, &stderr
					broken := &failAfterWriter{cause: errors.New("writer-private")}
					switch mode {
					case "result error":
						output = broken
					case "result short":
						output = shortWriter{}
					case "diagnostic error":
						diagnostic = broken
					case "diagnostic short":
						diagnostic = shortWriter{}
					}
					command.SetOut(output)
					command.SetErr(diagnostic)
					command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n"+line+"\nGET http://example.test/next")})
					code := command.ExecuteContext(context.Background())
					// A diagnostic writer is unused on success; in that case both
					// entries complete normally. All actual writes must surface 2.
					success := line == `jsonpath "$" exists` && body != "invalid" || line == `jsonpath "$" > 1` && body == "2"
					wantCode, wantCalls := 2, 1
					if strings.HasPrefix(mode, "diagnostic") && success {
						wantCode, wantCalls = 0, 2
					}
					if code != wantCode || calls != wantCalls || strings.Contains(out.String()+stderr.String()+broken.String(), "writer-private") {
						t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out.String(), stderr.String())
					}
				})
			}
		}
	}
}

func TestJSONLimitsPendingSecretsCLI(t *testing.T) {
	for _, line := range []string{`jsonpath "$.missing" exists`, `jsonpath "$.items" count == 987654321`, `jsonpath "$.number" > 987654321`, `jsonpath "$.private" > 987654321`} {
		code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			return runner.Response{Status: 200, Body: []byte(`{"items":["body-private"],"number":123456789,"private":"actual-private"}`)}, nil
		}), "test", writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n"+line+"\n[Captures]\ntoken: jsonpath \"$.missing.bad\"\nGET http://example.test/next"), "--secret", "token")
		if code != 1 || !strings.Contains(stderr, "esperado [oculto], recebido [oculto]") || !strings.Contains(out, "não_iniciadas=1") {
			t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
		}
		for _, secret := range []string{"987654321", "123456789", "body-private", "actual-private"} {
			if strings.Contains(out+stderr, secret) {
				t.Fatalf("leaked %s: %s %s", secret, out, stderr)
			}
		}
	}
}
