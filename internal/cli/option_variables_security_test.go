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

// GO-582: session pre-registration reaches invocation writers and both reports,
// including literals discarded before publication and literals in future entries.
func TestOptionVariableSecretOutputs(t *testing.T) {
	t.Setenv("HTTPGO_OPTION_SECRET", "initial-private")
	base := writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_OPTION_SECRET}'}")
	for _, format := range []string{"text", "verbose", "junit", "html"} {
		for _, success := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", format, success), func(t *testing.T) {
				first := writeScenario(t, "GET http://example.test/{{token}}\n[Options]\nvariable: token=discarded-private\nvariable: token=assigned-private\nHTTP 200\nGET http://example.test\n[Options]\nvariable: token=future-private\nHTTP 200")
				second := writeScenario(t, "GET http://example.test/{{token}}\nHTTP 200\n[Asserts]\njsonpath \"$.echo\" == \"different\"")
				calls := 0
				fake := executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
					calls++
					want := "http://example.test/initial-private"
					if calls == 1 {
						want = "http://example.test/assigned-private"
					} else if success && calls == 2 {
						want = "http://example.test"
					}
					if req.URL != want {
						t.Fatalf("request=%+v", req)
					}
					if calls == 1 && !success {
						return runner.Response{}, errors.New("assigned-private transport discarded-private")
					}
					return runner.Response{Status: 200, Body: []byte(`{"echo":"initial-private discarded-private assigned-private future-private"}`)}, nil
				})
				args := []string{"test", first, second, "--config", base}
				path := filepath.Join(t.TempDir(), "report."+format)
				switch format {
				case "verbose":
					args = append(args, "-vv")
				case "junit", "html":
					args = append(args, "--output", format, "--output-file", path)
				}
				code, out, stderr := execute(t, context.Background(), fake, args...)
				wantCode, wantCalls := 1, 3
				if !success {
					wantCode, wantCalls = 2, 2 // Continue to the next file after transport error.
				}
				if code != wantCode || calls != wantCalls {
					t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
				}
				if format == "html" {
					_, html := readHTML(t, path)
					out += html
				}
				if format == "junit" {
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					out += string(data)
				}
				for _, value := range []string{"initial-private", "discarded-private", "assigned-private", "future-private"} {
					if strings.Contains(out+stderr, value) {
						t.Fatalf("secret %s exposed: out=%s err=%s", value, out, stderr)
					}
				}
			})
		}
	}
}

func TestOptionVariableWriterFailures(t *testing.T) {
	for _, stream := range []string{"result", "diagnostic"} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/short=%v", stream, short), func(t *testing.T) {
				calls := 0
				cmd := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					status := 200
					if stream == "diagnostic" {
						status = 503
					}
					return runner.Response{Status: status}, nil
				}), BuildInfo{})
				cmd.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/{{token}}\n[Options]\nvariable: token=discarded-private\nvariable: token=assigned-private\nHTTP 200\nGET http://example.test/never\n[Options]\nvariable: token=next-private"), "--secret", "token"})
				writer := &failAfterWriter{cause: errors.New("writer assigned-private")}
				var out, stderr bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(&stderr)
				if stream == "result" {
					cmd.SetOut(writer)
					if short {
						cmd.SetOut(shortWriter{})
					}
				} else {
					cmd.SetErr(writer)
					if short {
						cmd.SetErr(shortWriter{})
					}
				}
				code := cmd.ExecuteContext(context.Background())
				if code != 2 || calls != 1 || !short && writer.calls != 1 || strings.Contains(out.String()+stderr.String()+writer.String(), "private") {
					t.Fatalf("code=%d calls=%d writes=%d out=%s err=%s", code, calls, writer.calls, &out, &stderr)
				}
			})
		}
	}
}

// GO-583: fail-fast only changes whether subsequent files run; reusing the root
// command gets a new invocation, never the previous file's assigned values.
func TestOptionVariableSuiteFailureAndReuse(t *testing.T) {
	for _, failFast := range []bool{false, true} {
		var out, stderr bytes.Buffer
		calls := 0
		cmd := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
			calls++
			if req.URL == "http://example.test/assigned" {
				return runner.Response{Status: 503}, nil
			}
			if req.URL != "http://example.test/initial" {
				t.Fatalf("request=%+v", req)
			}
			return runner.Response{Status: 200}, nil
		}), BuildInfo{})
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		first := writeScenario(t, "GET http://example.test/{{id}}\n[Options]\nvariable: id=assigned\nHTTP 200\nGET http://example.test/never")
		second := writeScenario(t, "GET http://example.test/{{id}}\nHTTP 200")
		args := []string{"test", first, second, "--var", "id=initial"}
		if failFast {
			args = append(args, "--fail-fast")
		}
		cmd.SetArgs(args)
		code := cmd.ExecuteContext(context.Background())
		if code != 1 || calls != map[bool]int{false: 2, true: 1}[failFast] {
			t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, &out, &stderr)
		}
		out.Reset()
		stderr.Reset()
		calls = 0
		cmd.SetArgs([]string{"test", second, "--var", "id=initial"})
		if code := cmd.ExecuteContext(context.Background()); code != 0 || calls != 1 || stderr.Len() != 0 {
			t.Fatalf("reuse code=%d calls=%d out=%s err=%s", code, calls, &out, &stderr)
		}
	}
}
