package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestProjectionWriterFailureOverridesOutcomes(t *testing.T) {
	for _, body := range []string{`[]`, `["forbidden","second"]`, `{}`, `[9007199254740992,9007199254740993,9223372036854775809,1e-99999999999999999999999]`} {
		for _, short := range []bool{false, true} {
			calls := 0
			c := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(body)}, nil
			}), BuildInfo{})
			c.SetArgs([]string{"test", writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$[*]\" not contains \"forbidden\"\nGET http://example.test/never")})
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

func TestProjectionProtectedSummariesCLI(t *testing.T) {
	for _, pending := range []bool{false, true} {
		source := "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$[*]\" count == 0"
		if pending {
			source += "\n[Captures]\nfuture: jsonpath \"$\""
		}
		code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			return runner.Response{Status: 200, Body: []byte(`["known-private","unrelated-private"]`)}, nil
		}), "test", writeScenario(t, source), "--var", "token=known-private", "--secret", "token", "--secret", "future")
		if code != 1 || !strings.Contains(stderr, "[oculto]") || strings.Contains(out+stderr, "contagem=") || strings.Contains(out+stderr, "known-private") || strings.Contains(out+stderr, "unrelated-private") || strings.Contains(stderr, "erro de captura") {
			t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
		}
	}
}

// A failed batch still protects discovered leaves across the suite, but cannot
// publish any capture into the next file or the next command invocation.
func TestProjectionSuiteProtectionAndIsolation(t *testing.T) {
	const secret = "discovered-private"
	dir := t.TempDir()
	suiteFile(t, dir, "01.hurl", "GET http://example.test/capture\n[Captures]\nx: jsonpath \"$[*]\"\nmissing: jsonpath \"$.none\"")
	suiteFile(t, dir, "02-"+secret+".hurl", "GET http://example.test/echo\nHTTP *\n[Asserts]\njsonpath \"$[*]\" count == 0")
	suiteFile(t, dir, "03.hurl", "GET http://example.test/{{x}}")
	calls := 0
	c := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		return runner.Response{Status: 200, Body: []byte(`["` + secret + `","second"]`)}, nil
	}), BuildInfo{})
	var out, stderr bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&stderr)
	c.SetArgs([]string{"test", dir, "--secret", "x"})
	if code := c.ExecuteContext(context.Background()); code != 2 || calls != 2 || strings.Contains(out.String()+stderr.String(), secret) || strings.Contains(out.String(), "contagem=") {
		t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	c.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/"+secret)})
	if code := c.ExecuteContext(context.Background()); code != 0 || !strings.Contains(out.String(), secret) {
		t.Fatalf("state leaked: %d %s", code, out.String())
	}
}
