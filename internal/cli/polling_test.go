package cli

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestPollingCLIAndSuite(t *testing.T) {
	for _, ready := range []bool{true, false} {
		t.Run(fmt.Sprint(ready), func(t *testing.T) {
			calls := 0
			fake := executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
				calls++
				if req.URL == "http://example.test/job" && (!ready || calls < 3) {
					return runner.Response{Status: 200, Body: []byte(`{"status":"PENDING"}`)}, nil
				}
				return runner.Response{Status: 200, Body: []byte(`{"status":"READY"}`)}, nil
			})
			file := writeScenario(t, "GET http://example.test/job\n[Options]\nretry: 3\nretry-interval: 0\nHTTP 200\n[Asserts]\njsonpath \"$.status\" == \"READY\"\nGET http://example.test/next\nHTTP 200\n")
			code, out, stderr := execute(t, context.Background(), fake, "test", file)
			wantCode := 0
			if !ready {
				wantCode = 1
			}
			if code != wantCode || calls != 4 || strings.Count(out, "GET ") != map[bool]int{true: 2, false: 1}[ready] || !strings.Contains(out, "tentativas_iniciadas=4 respostas_completas=4 status_comparados=4") {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
			if ready && (stderr != "" || strings.Contains(out, "PENDING") || !strings.Contains(out, "iniciadas=2 concluídas=2 validadas=2") || !strings.Contains(out, "requisições_falha_assertion=0")) {
				t.Fatalf("out=%s err=%s", out, stderr)
			}
			if !ready && (!strings.Contains(out, "[tentativas=4]") || !strings.Contains(stderr, "PENDING") || !strings.Contains(out, "não_iniciadas=1")) {
				t.Fatalf("out=%s err=%s", out, stderr)
			}
		})
	}
}

func TestPollingInvalidLateOptionsCLI(t *testing.T) {
	for _, option := range []string{"retry: -1", "retry: 1.5", "retry: 1\nretry: 2", "retry-interval: 9223372036855", "timeout: 1"} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/job\n[Options]\n"+option+"\nHTTP 200\n"))
		if code != 2 || len(fake.calls) != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, fmt.Sprintf(":%d:1:", 5+strings.Count(option, "\n"))) {
			t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
		}
	}
}
