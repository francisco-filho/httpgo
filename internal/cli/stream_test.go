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

type executorFunc func(context.Context, runner.Request) (runner.Response, error)

func (f executorFunc) Execute(ctx context.Context, request runner.Request) (runner.Response, error) {
	return f(ctx, request)
}

func TestResultsAreWrittenBeforeTheNextRequest(t *testing.T) {
	const total = 40
	var input strings.Builder
	for i := range total {
		_, _ = fmt.Fprintf(&input, "GET http://url-user:url-password@example.test/step/%d?token=query-secret#fragment-secret\n", i)
		if i%2 == 0 {
			input.WriteString("HTTP 200\n")
		}
	}
	var out, stderr bytes.Buffer
	calls := 0
	executor := executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
		if got := strings.Count(out.String(), "GET "); got != calls {
			t.Fatalf("request %d started with only %d results visible: %q", calls+1, got, out.String())
		}
		if strings.Contains(out.String(), "resumo") {
			t.Fatalf("premature summary: %q", out.String())
		}
		if !strings.Contains(request.URL, "url-password") || !strings.Contains(request.URL, "query-secret") {
			t.Fatal("operational URL was sanitized")
		}
		status := 500
		if calls%2 == 0 {
			status = 200
		}
		calls++
		return runner.Response{Status: status}, nil
	})
	command := NewRootCommand(executor, BuildInfo{})
	command.SetArgs([]string{"test", writeScenario(t, input.String())})
	command.SetOut(&out)
	command.SetErr(&stderr)
	if code := command.ExecuteContext(context.Background()); code != 0 || calls != total || stderr.Len() != 0 {
		t.Fatalf("code=%d,calls=%d,stdout=%q,stderr=%q", code, calls, out.String(), stderr.String())
	}
	if strings.Count(out.String(), "GET ") != total || strings.Count(out.String(), "resumo ") != 1 || strings.Count(out.String(), "Cenários executados: 1\n") != 1 || !strings.Contains(out.String(), "Testes executados: 40\n") || !strings.Contains(out.String(), "validadas=20 sem_validação=20 não_iniciadas=0") {
		t.Fatalf("incorrect results or final summary: %q", out.String())
	}
	for _, secret := range []string{"url-user", "url-password", "query-secret", "fragment-secret"} {
		if strings.Contains(out.String()+stderr.String(), secret) {
			t.Errorf("leaked %q", secret)
		}
	}
}

type failAfterWriter struct {
	bytes.Buffer
	successes, calls int
	cause            error
}

func (w *failAfterWriter) Write(data []byte) (int, error) {
	w.calls++
	if w.calls > w.successes {
		return 0, w.cause
	}
	return w.Buffer.Write(data)
}

// Do not inherit bytes.Buffer.WriteString: io.WriteString must exercise Write.
func (w *failAfterWriter) WriteString(text string) (int, error) { return w.Write([]byte(text)) }

func TestStreamingWriteFailureStopsRequests(t *testing.T) {
	transportErr := errors.New("transport-secret")
	for _, tc := range []struct {
		name                                string
		successfulWrites, status, wantCalls int
		err                                 error
	}{
		{"first result", 0, 200, 1, nil},
		{"later result", 1, 200, 2, nil},
		{"summary", 3, 200, 3, nil},
		{"divergent result", 0, 503, 1, nil},
		{"transport failure", 0, 200, 1, transportErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := writeScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/second\nHTTP 200\nGET http://example.test/third\nHTTP 200")
			calls := 0
			executor := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: tc.status}, tc.err
			})
			out := &failAfterWriter{successes: tc.successfulWrites, cause: errors.New("write-secret")}
			var stderr bytes.Buffer
			command := NewRootCommand(executor, BuildInfo{})
			command.SetArgs([]string{"test", file})
			command.SetOut(out)
			command.SetErr(&stderr)
			code := command.ExecuteContext(context.Background())
			if code != 2 || calls != tc.wantCalls || out.calls != tc.successfulWrites+1 || !strings.Contains(stderr.String(), "escrever o resultado") {
				t.Fatalf("code=%d,requests=%d,writes=%d,stdout=%q,stderr=%q", code, calls, out.calls, out.String(), stderr.String())
			}
			if strings.Contains(out.String(), "resumo") || strings.Count(out.String(), "GET ") != tc.successfulWrites {
				t.Fatalf("unexpected output after write failure: %q", out.String())
			}
			if tc.name != "summary" && !strings.Contains(stderr.String(), fmt.Sprintf(":%d:1:", 2*(calls-1)+1)) {
				t.Fatalf("missing failure position: %q", stderr.String())
			}
			for _, secret := range []string{"write-secret", "transport-secret"} {
				if strings.Contains(out.String()+stderr.String(), secret) {
					t.Errorf("leaked %q", secret)
				}
			}
		})
	}
}
