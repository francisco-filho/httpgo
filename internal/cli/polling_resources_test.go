package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestPollingTerminalWriteFailure(t *testing.T) {
	for _, ready := range []bool{true, false} {
		for _, mode := range []string{"result", "short", "file-summary", "suite-summary"} {
			t.Run(fmt.Sprintf("%v/%s", ready, mode), func(t *testing.T) {
				calls := 0
				executor := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					status := 503
					if ready && calls == 3 {
						status = 200
					}
					return runner.Response{Status: status}, nil
				})
				out := &failAfterWriter{cause: errors.New("private write failure")}
				switch mode {
				case "file-summary":
					out.successes = 1
				case "suite-summary":
					out.successes = 2
				}
				var stderr bytes.Buffer
				cmd := NewRootCommand(executor, BuildInfo{})
				// A later file must not start after an entry/file-summary write failure.
				file := writeScenario(t, "GET http://example.test/job\n[Options]\nretry: 2\nretry-interval: 0\nHTTP 200\n")
				cmd.SetArgs([]string{"test", file})
				cmd.SetOut(out)
				if mode == "short" {
					cmd.SetOut(shortWriter{})
				}
				cmd.SetErr(&stderr)
				code := cmd.ExecuteContext(context.Background())
				if code != 2 || calls != 3 || strings.Contains(stderr.String(), "private") || mode != "short" && out.calls != out.successes+1 {
					t.Fatalf("code=%d calls=%d writes=%d out=%s err=%s", code, calls, out.calls, out.String(), stderr.String())
				}
			})
		}
	}
}

func TestPollingDiscardedSecretProtectsFollowingFile(t *testing.T) {
	secret := "discarded-private-session"
	first := writeScenario(t, "GET http://example.test/job\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200\n")
	second := writeScenario(t, "GET http://example.test/next\nHTTP 200\n[Asserts]\njsonpath \"$.value\" == \"other\"\n")
	calls := 0
	code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		if calls == 1 {
			return runner.Response{Status: 503, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: secret}}}, nil
		}
		return runner.Response{Status: 200, Body: []byte(`{"value":"` + secret + `"}`)}, nil
	}), "test", first, second)
	if code != 1 || calls != 3 || strings.Contains(out+stderr, secret) || !strings.Contains(out+stderr, "[oculto]") {
		t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
	}
}
