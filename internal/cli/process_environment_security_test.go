package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestProcessEnvironmentEarlyProtection(t *testing.T) {
	const token = "fiction-process-sensitive"
	t.Setenv("HTTPGO_US13_AVAILABLE", token)
	for _, mode := range []string{"arguments", "CLI duplicate", "CLI hierarchy", "syntax", "missing file", "missing reference first", "missing reference last", "merge conflict"} {
		t.Run(mode, func(t *testing.T) {
			base := writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_US13_AVAILABLE}', structure: 1}")
			file := filepath.Join(t.TempDir(), token+".hurl")
			text := "GET http://example.test/{{token}}\nHTTP 200"
			if mode == "syntax" {
				text += "\nHEAD http://example.test/"
			}
			if mode != "missing file" {
				if err := os.WriteFile(file, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"test", file, "--config", base}
			switch mode {
			case "arguments":
				args = append(args, "--unknown")
			case "CLI duplicate":
				args = append(args, "--var", "token=first-fiction", "--var", "token=second-fiction")
			case "CLI hierarchy":
				args = append(args, "--var", "structure.child=1")
			case "missing reference first", "missing reference last":
				key := "aMissing"
				if mode == "missing reference last" {
					key = "zMissing"
				}
				// Known value occurs in diagnostic metadata, not just in scenario.
				base = writeConfig(t, fmt.Sprintf("version: 1\nvariables: {token: '${HTTPGO_US13_AVAILABLE}', %s: '${HTTPGO_US13_ABSENT}'}", key))
				renamed := filepath.Join(filepath.Dir(base), token+".yaml")
				if err := os.Rename(base, renamed); err != nil {
					t.Fatal(err)
				}
				args = []string{"test", file, "--config", renamed}
			case "merge conflict":
				base = filepath.Join(t.TempDir(), token+".yaml")
				if err := os.WriteFile(base, []byte("version: 1\nvariables: {token: '${DISCARDED}', structure: 1}"), 0600); err != nil {
					t.Fatal(err)
				}
				writeEnvironment(t, base, "local", "variables: {token: '${HTTPGO_US13_AVAILABLE}', structure: {child: true}}")
				// Environment name is in the diagnostic path, so put the token there.
				old := filepath.Join(filepath.Dir(base), "environments", "local.yaml")
				if err := os.Rename(old, filepath.Join(filepath.Dir(old), token+".yaml")); err != nil {
					t.Fatal(err)
				}
				args = []string{"test", file, "--config", base, "--env", token}
			}
			fake := &fakeExecutor{}
			code, out, stderr := execute(t, context.Background(), fake, args...)
			if code != 2 || (mode == "syntax" && !prevalidationSummary(out) || mode != "syntax" && out != "") || stderr == "" || len(fake.calls) != 0 {
				t.Fatalf("code=%d calls=%d", code, len(fake.calls))
			}
			for _, value := range []string{token, "first-fiction", "second-fiction"} {
				if strings.Contains(out+stderr, value) {
					t.Fatalf("early protection failed in %s", mode)
				}
			}
		})
	}
}

func TestProcessEnvironmentEchoAssertions(t *testing.T) {
	const token = "fiction-echo-token"
	t.Setenv("HTTPGO_US13_TOKEN", token)
	for _, mode := range []string{"header actual", "header expected", "json actual", "json expected"} {
		t.Run(mode, func(t *testing.T) {
			expected, actual := "different", "prefix-"+token
			if strings.Contains(mode, "expected") {
				expected, actual = actual, expected
			}
			assertion := fmt.Sprintf("header \"X-Echo\" == %q", expected)
			if strings.HasPrefix(mode, "json") {
				assertion = fmt.Sprintf("jsonpath \"$.value\" == %q", expected)
			}
			calls := 0
			fake := executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
				calls++
				if request.URL != "http://example.test/"+token {
					t.Error("operational value changed")
				}
				return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "X-Echo", Value: actual}}, Body: []byte(fmt.Sprintf(`{"value":%q}`, actual))}, nil
			})
			code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/{{token}}\nHTTP 200\n[Asserts]\n"+assertion+"\nGET http://example.test/never"), "--config", writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_US13_TOKEN}'}"))
			if code != 1 || calls != 1 || strings.Contains(out+stderr, token) || !strings.Contains(out, "[oculto]") || !strings.Contains(stderr, "[oculto]") {
				t.Fatalf("mode=%s code=%d calls=%d protected=%v", mode, code, calls, !strings.Contains(out+stderr, token))
			}
		})
	}
}

func TestProcessEnvironmentReuseIsolation(t *testing.T) {
	base := writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_US13_TOKEN}'}\nsecrets: [future]")
	file := writeScenario(t, "GET http://example.test/{{token}}\nHTTP 200\n[Captures]\nfuture: jsonpath \"$.value\"\nGET http://example.test/{{future}}\nHTTP 200")
	want := ""
	calls := 0
	command := NewRootCommand(executorFunc(func(_ context.Context, r runner.Request) (runner.Response, error) {
		calls++
		if calls%2 == 1 && r.URL != "http://example.test/"+want {
			t.Error("stale process value")
		}
		return runner.Response{Status: 200, Body: []byte(`{"value":"captured-fiction"}`)}, nil
	}), BuildInfo{})
	var out, stderr bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&stderr)
	command.SetArgs([]string{"test", file, "--config", base})
	for _, value := range []string{"first-fiction", "second-fiction"} {
		t.Setenv("HTTPGO_US13_TOKEN", value)
		want = value
		out.Reset()
		stderr.Reset()
		if code := command.ExecuteContext(context.Background()); code != 0 || strings.Contains(out.String()+stderr.String(), value) || strings.Contains(out.String(), "captured-fiction") {
			t.Fatalf("code=%d protection/isolation failed", code)
		}
	}
	out.Reset()
	stderr.Reset()
	command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/{{future}}"), "--config", base})
	if code := command.ExecuteContext(context.Background()); code != 2 || calls != 4 {
		t.Fatalf("capture leaked; code=%d calls=%d", code, calls)
	}
	out.Reset()
	stderr.Reset()
	command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/second-fiction\nHTTP 200"), "--config", writeConfig(t, "")})
	want = "second-fiction"
	if code := command.ExecuteContext(context.Background()); code != 0 || !strings.Contains(out.String(), want) {
		t.Fatalf("stale marks; code=%d", code)
	}
}

func TestProcessEnvironmentOutputAndTransportFailures(t *testing.T) {
	t.Setenv("HTTPGO_US13_TOKEN", "fiction-failure-token")
	for _, mode := range []string{"transport", "result error", "result short", "diagnostic error", "diagnostic short", "cancel before send"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				if mode == "transport" {
					return runner.Response{}, errors.New("fiction-failure-token")
				}
				return runner.Response{Status: 503}, nil
			}), BuildInfo{})
			var out, stderr bytes.Buffer
			var output, diagnostic io.Writer = &out, &stderr
			broken := &failAfterWriter{cause: errors.New("fiction-failure-token")}
			switch mode {
			case "result error":
				output = broken
			case "result short":
				output = shortWriter{}
			case "diagnostic error":
				diagnostic = broken
			case "diagnostic short":
				diagnostic = shortWriter{}
			case "cancel before send":
				cancel()
			}
			command.SetOut(output)
			command.SetErr(diagnostic)
			command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/{{token}}\nHTTP 200\nGET http://example.test/never"), "--config", writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_US13_TOKEN}'}")})
			code := command.ExecuteContext(ctx)
			wantCalls := 1
			if mode == "cancel before send" {
				wantCalls = 0
			}
			if code != 2 || calls != wantCalls || strings.Contains(out.String()+stderr.String()+broken.String(), "fiction-failure-token") {
				t.Fatalf("mode=%s code=%d calls=%d", mode, code, calls)
			}
		})
	}
}
