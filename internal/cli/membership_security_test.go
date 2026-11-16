package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestMembershipWriterFailureOverridesAllOutcomes(t *testing.T) {
	for _, body := range []string{`[]`, `["forbidden"]`, `invalid`} {
		for _, short := range []bool{false, true} {
			calls := 0
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(body)}, nil
			}), BuildInfo{})
			command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$\" not contains \"forbidden\"\nGET http://example.test/never")})
			var stderr bytes.Buffer
			command.SetErr(&stderr)
			writer := &failAfterWriter{cause: errors.New("write-private")}
			if short {
				command.SetOut(shortWriter{})
			} else {
				command.SetOut(writer)
			}
			if code := command.ExecuteContext(context.Background()); code != 2 || calls != 1 || !strings.Contains(stderr.String(), "escrever o resultado") || strings.Contains(stderr.String(), "write-private") {
				t.Fatalf("code=%d calls=%d err=%s", code, calls, stderr.String())
			}
			if !short && writer.calls != 1 {
				t.Fatalf("writer retried %d times", writer.calls)
			}
		}
	}
}

func TestMembershipSecretOperandsCLI(t *testing.T) {
	for _, pending := range []bool{false, true} {
		source := "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$\" contains \"expected-private\""
		if pending {
			source += "\n[Captures]\ntoken: jsonpath \"$.missing.bad\""
		}
		source += "\nGET http://example.test/never"
		calls := 0
		code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			calls++
			return runner.Response{Status: 200, Body: []byte(`["actual-private",{"other":"body-private"}]`)}, nil
		}), "test", writeScenario(t, source), "--var", "initial=expected-private", "--secret", "initial", "--secret", "token")
		if code != 1 || calls != 1 || !strings.Contains(stderr, "[oculto]") || strings.Contains(stderr, "erro de captura") {
			t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
		}
		for _, secret := range []string{"expected-private", "actual-private", "body-private"} {
			if strings.Contains(out+stderr, secret) {
				t.Fatalf("leaked %s", secret)
			}
		}
	}
}
