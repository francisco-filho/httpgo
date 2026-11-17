package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestJSONTypeWriterFailureOverridesOutcomes(t *testing.T) {
	for _, body := range []string{`[]`, `["value-private"]`, `invalid`} {
		for _, short := range []bool{false, true} {
			calls := 0
			c := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(body)}, nil
			}), BuildInfo{})
			c.SetArgs([]string{"test", writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$\" isEmpty\nGET http://example.test/never")})
			var stderr bytes.Buffer
			c.SetErr(&stderr)
			writer := &failAfterWriter{cause: errors.New("write-private")}
			if short {
				c.SetOut(shortWriter{})
			} else {
				c.SetOut(writer)
			}
			code := c.ExecuteContext(context.Background())
			if code != 2 || calls != 1 || !strings.Contains(stderr.String(), "escrever o resultado") || strings.Contains(stderr.String(), "write-private") || strings.Contains(stderr.String(), "value-private") || !short && writer.calls != 1 {
				t.Fatalf("code=%d calls=%d err=%s", code, calls, stderr.String())
			}
		}
	}
}

func TestJSONTypeProtectedMetadataCLI(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for _, expression := range []string{`jsonpath "$" isString`, `jsonpath "$.absent" isInteger`} {
			source := "GET http://example.test\nHTTP *\n[Asserts]\n" + expression
			if pending {
				source += "\n[Captures]\nfuture: jsonpath \"$.none.bad\""
			}
			code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				return runner.Response{Status: 200, Body: []byte(`{"v":["known-private"]}`)}, nil
			}), "test", writeScenario(t, source), "--var", "token=known-private", "--secret", "token", "--secret", "future")
			if code != 1 || strings.Contains(out+stderr, "known-private") || strings.Contains(stderr, "erro de captura") || strings.Contains(stderr, "esperado") {
				t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
			}
			if pending || strings.Contains(expression, `"$"`) {
				if !strings.Contains(stderr, "[oculto]") {
					t.Fatal(stderr)
				}
			}
			if strings.Contains(expression, "absent") && !strings.Contains(stderr, "ausente") {
				t.Fatal(stderr)
			}
		}
	}
}

func TestJSONTypeSuiteProtectionAndIsolation(t *testing.T) {
	const secret = "discovered-private"
	dir := t.TempDir()
	suiteFile(t, dir, "01.hurl", "GET http://example.test/capture\n[Captures]\nx: jsonpath \"$[*]\"\nmissing: jsonpath \"$.none\"")
	suiteFile(t, dir, "02-"+secret+".hurl", "GET http://example.test/check\nHTTP *\n[Asserts]\njsonpath \"$\" isString")
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
	if code := c.ExecuteContext(context.Background()); code != 2 || calls != 2 || strings.Contains(out.String()+stderr.String(), secret) || !strings.Contains(stderr.String(), "[oculto]") {
		t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	c.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/"+secret)})
	if code := c.ExecuteContext(context.Background()); code != 0 || !strings.Contains(out.String(), secret) {
		t.Fatalf("state leaked: %d %s", code, out.String())
	}
}
