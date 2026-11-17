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

	"capimgrosso.com/httpgo/internal/runner"
)

// GO-402,414: a reused command never retains the opt-in; disabled parsing rejects
// the entire file without network, even when the variable is already supplied.
func TestDynamicFilterInvocationIsolation(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"01.hurl", "02.hurl"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("GET http://example.test\nHTTP 200\n[Asserts]\n"+`jsonpath "$[?(@.key=='{{key}}')].id" == 1`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	cmd := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		return runner.Response{Status: 200, Body: []byte(`[{"key":"selected","id":1}]`)}, nil
	}), BuildInfo{})
	for _, enabled := range []bool{true, false, true, false} {
		args := []string{"test", dir, "--var", "key=selected"}
		want := 2
		if enabled {
			args = append(args, "--render-asserts")
			want = 0
		}
		var out, stderr bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		if code := cmd.ExecuteContext(context.Background()); code != want || enabled && !strings.Contains(out.String(), "sucessos=2") {
			t.Fatalf("enabled=%v code=%d out=%s err=%s", enabled, code, out.String(), stderr.String())
		}
	}
	if calls != 4 {
		t.Fatalf("calls=%d", calls)
	}
}

// GO-409,413: failure/short write wins over success, divergence and binding errors;
// streams never include the operand, derived secret, substituted query or plan.
func TestDynamicFilterSecretWriterPriority(t *testing.T) {
	for _, mode := range []string{"success", "mismatch", "missing", "type"} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/short=%v", mode, short), func(t *testing.T) {
				value := `"fake-credential"`
				if mode == "type" {
					value = `{"nested":"fake-credential"}`
				}
				operand := "credential"
				if mode == "missing" {
					operand = "absent"
				}
				source := "GET http://example.test/capture\nHTTP 200\n[Captures]\ncredential: jsonpath \"$\"\nGET http://example.test/assert\nHTTP 200\n[Asserts]\njsonpath \"$[?(@.key=='prefix-{{" + operand + "}}')].id\" == 1\nGET http://example.test/never"
				file := writeScenario(t, source)
				calls := 0
				cmd := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					if calls == 1 {
						return runner.Response{Status: 200, Body: []byte(value)}, nil
					}
					id := 1
					if mode == "mismatch" {
						id = 2
					}
					return runner.Response{Status: 200, Body: []byte(fmt.Sprintf(`[{"key":"prefix-fake-credential","id":%d}]`, id))}, nil
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
				for _, bad := range []string{"fake-credential", "private writer cause", "{{credential}}"} {
					if strings.Contains(out.String()+stderr.String(), bad) {
						t.Fatalf("unsafe output: %s %s", out.String(), stderr.String())
					}
				}
			})
		}
	}
}
