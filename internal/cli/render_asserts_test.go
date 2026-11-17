package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

// GO-389: reused command has no sticky profile, and all files share only the flag.
func TestRenderAssertsInvocationIsolation(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"01.hurl", "02.hurl"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("GET http://example.test\nHTTP 200\n[Asserts]\njsonpath \"$\" == \"{{v}}\""), 0600); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	body := `"resolved"`
	cmd := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		return runner.Response{Status: 200, Body: []byte(body)}, nil
	}), BuildInfo{})
	for _, enabled := range []bool{true, false, true, false} {
		if enabled {
			body = `"resolved"`
		} else {
			body = `"{{v}}"`
		}
		args := []string{"test", dir, "--var", "v=resolved"}
		if enabled {
			args = append(args, "--render-asserts")
		}
		var out, stderr bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		if code := cmd.ExecuteContext(context.Background()); code != 0 || stderr.Len() != 0 || !strings.Contains(out.String(), "sucessos=2") {
			t.Fatalf("enabled=%v code=%d out=%s err=%s", enabled, code, out.String(), stderr.String())
		}
	}
	if calls != 8 {
		t.Fatalf("calls=%d", calls)
	}
	for _, arg := range []string{"--render-asserts=true", "--render-asserts=false", "--render-asserts="} {
		if _, err := parseArguments([]string{"test", dir, arg}); err == nil {
			t.Fatalf("accepted %s", arg)
		}
	}
}

// GO-390,395: writes keep category 2 priority over success, mismatch and expected errors.
func TestRenderAssertsSecretsAndWriterPriority(t *testing.T) {
	for _, kind := range []string{"JSON", "header", "body", "regex"} {
		for _, mode := range []string{"success", "mismatch", "expected"} {
			for _, short := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/short=%v", kind, mode, short), func(t *testing.T) {
					expression := `jsonpath "$" == "prefix-{{credential}}-suffix"`
					switch kind {
					case "header":
						expression = `header "X" == "prefix-{{credential}}-suffix"`
					case "body":
						expression = `body contains "prefix-{{credential}}-suffix"`
					case "regex":
						expression = `header "X" matches "^prefix-{{credential}}-suffix$"`
					}
					value := `"fake-credential"`
					if mode == "expected" {
						if kind == "regex" {
							value = `"[fake-credential"`
						} else {
							value = `{"synthetic-key":"fake-credential"}`
						}
					}
					file := writeScenario(t, "GET http://example.test/capture\nHTTP 200\n[Captures]\ncredential: jsonpath \"$\"\nGET http://example.test/assert\nHTTP 200\n[Asserts]\n"+expression+"\nGET http://example.test/never")
					calls := 0
					cmd := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
						calls++
						if calls == 1 {
							return runner.Response{Status: 200, Body: []byte(value)}, nil
						}
						text := "prefix-fake-credential-suffix"
						if mode == "mismatch" {
							text = "partial-private"
						}
						return runner.Response{Status: 200, Body: []byte(`"` + text + `"`), Headers: []query.HeaderValue{{Name: "X", Value: text}}}, nil
					}), BuildInfo{})
					out := &failAfterWriter{successes: 1, cause: errors.New("private writer cause")}
					if short {
						out.cause = nil
					}
					var stderr bytes.Buffer
					cmd.SetArgs([]string{"test", file, "--render-asserts", "--secret", "credential"})
					cmd.SetOut(out)
					cmd.SetErr(&stderr)
					code := cmd.ExecuteContext(context.Background())
					if code != 2 || calls != 2 || out.calls != 2 || strings.Contains(out.String(), "resumo") || !strings.Contains(stderr.String(), "escrever o resultado") {
						t.Fatalf("code=%d calls=%d writes=%d out=%s err=%s", code, calls, out.calls, out.String(), stderr.String())
					}
					for _, bad := range []string{"fake-credential", "partial-private", "private writer cause", "{{credential}}"} {
						if strings.Contains(out.String()+stderr.String(), bad) {
							t.Fatalf("unsafe output: %s %s", out.String(), stderr.String())
						}
					}
				})
			}
		}
	}
}
