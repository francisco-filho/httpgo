package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestHeaderMatchesWriterFailureOverridesAllOutcomes(t *testing.T) {
	for _, mode := range []string{"pass", "mismatch", "execution"} {
		for _, short := range []bool{false, true} {
			for _, stream := range []string{"stdout", "stderr"} {
				// Success has no diagnostic on stderr; that writer cannot fail here.
				if mode == "pass" && stream == "stderr" {
					continue
				}
				calls := 0
				command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					value := "expected-private"
					if mode == "mismatch" {
						value = "actual-private"
					}
					var err error
					if mode == "execution" && calls == 2 {
						err = errors.New("transport-private")
					}
					return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: value}}}, err
				}), BuildInfo{})
				command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/prior\nHTTP 200\nGET http://example.test/check\nHTTP 200\n[Asserts]\nheader \"Set-Cookie\" matches \"expected-private\"\nGET http://example.test/never")})
				var out, stderr bytes.Buffer
				command.SetOut(&out)
				command.SetErr(&stderr)
				successes := 0
				if stream == "stdout" {
					successes = 1
				}
				writer := &failAfterWriter{successes: successes, cause: errors.New("write-private")}
				if stream == "stderr" {
					if short {
						command.SetErr(shortWriter{})
					} else {
						command.SetErr(writer)
					}
				} else {
					// A short write on the first result must also prevent the second request.
					if short {
						command.SetOut(shortWriter{})
					} else {
						command.SetOut(writer)
					}
				}
				code := command.ExecuteContext(context.Background())
				wantCalls := 2
				if short && stream == "stdout" {
					wantCalls = 1
				}
				if code != 2 || calls != wantCalls || (!short && writer.calls != successes+1) {
					t.Fatalf("mode=%s stream=%s short=%v code=%d calls=%d writes=%d out=%s err=%s", mode, stream, short, code, calls, writer.calls, out.String(), stderr.String())
				}
				combined := out.String() + stderr.String() + writer.String()
				if strings.Contains(combined, "private") || strings.Contains(writer.String(), "resumo") {
					t.Fatalf("unsafe or retried output: %s", combined)
				}
			}
		}
	}
}
