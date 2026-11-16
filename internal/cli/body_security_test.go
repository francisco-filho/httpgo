package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestBodySecretsAndPendingCapturesCLI(t *testing.T) {
	for _, pending := range []bool{false, true} {
		calls := 0
		source := "GET http://example.test\nHTTP 200\n[Asserts]\nbody contains \"expected-private\"\nGET http://example.test/next"
		if pending {
			source = strings.Replace(source, "\n[Asserts]", "\n[Captures]\ntoken: jsonpath \"$.missing.bad\"\n[Asserts]", 1)
		}
		code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			calls++
			return runner.Response{Status: 200, Body: []byte("actual-private body-private")}, nil
		}), "test", writeScenario(t, source), "--var", "initial=expected-private", "--secret", "initial", "--secret", "token")
		if code != 1 || calls != 1 || !strings.Contains(stderr, "[oculto]") || strings.Contains(stderr, "erro de captura") {
			t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
		}
		for _, forbidden := range []string{"expected-private", "actual-private", "body-private"} {
			if strings.Contains(out+stderr, forbidden) {
				t.Fatalf("leaked %q", forbidden)
			}
		}
	}
}

func TestBodyWriterFailureOverridesAllOutcomes(t *testing.T) {
	for _, body := range []string{"expected", "different", "\xff"} {
		for _, short := range []bool{false, true} {
			calls := 0
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(body)}, nil
			}), BuildInfo{})
			command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\nbody contains \"expected\"\nGET http://example.test/never")})
			var stderr bytes.Buffer
			command.SetErr(&stderr)
			writer := &failAfterWriter{cause: errors.New("write-private")}
			if short {
				command.SetOut(shortWriter{})
			} else {
				command.SetOut(writer)
			}
			if code := command.ExecuteContext(context.Background()); code != 2 || calls != 1 || !strings.Contains(stderr.String(), "escrever o resultado") || strings.Contains(stderr.String(), "write-private") {
				t.Fatalf("code=%d calls=%d stderr=%s", code, calls, stderr.String())
			}
			if !short && writer.calls != 1 {
				t.Fatalf("writer retried %d times", writer.calls)
			}
		}
	}
}
