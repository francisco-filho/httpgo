package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

func TestTypedCaptureProtectionAndStreams(t *testing.T) {
	for _, tc := range []struct{ raw, part string }{
		{`9007199254740993`, "9007199254740993"}, {`1.2300e+99`, "1.2300e+99"}, {`true`, "true"}, {`false`, "false"}, {`null`, "null"}, {`[]`, "[]"}, {`{}`, "{}"},
		{`{"privateKey":["hidden\"\\\n雪",9007199254740993]}`, "hidden\"\\\n雪"},
		{`["hidden/a b%?=雪",9007199254740993]`, "hidden/a b%?=雪"},
	} {
		for _, mode := range []string{"success", "status", "capture", "conflict", "render", "transport", "write", "short", "write-mismatch"} {
			t.Run(tc.raw+mode, func(t *testing.T) {
				source := "GET http://example.test/capture\nHTTP 200\n[Captures]\nsecret: jsonpath \"$\"\n"
				switch mode {
				case "capture":
					source += "missing: jsonpath \"$.absent\"\n"
				case "conflict":
					source += "secret.child: jsonpath \"$\"\n"
				}
				// Escaped constituent in static URL and filename checks dynamic protection
				// independently of placeholder provenance and of the JSON body.
				source += "POST http://example.test/" + url.PathEscape(tc.part) + "\nContent-Type: application/json\n{{secret}}\nHTTP 200\n"
				if mode == "render" {
					source += "GET http://example.test/{{absent}}\n"
				}
				source += "GET http://example.test/last"
				file := filepath.Join(t.TempDir(), "flow-"+url.QueryEscape(tc.part)+".hurl")
				if err := os.WriteFile(file, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				var out, stderr bytes.Buffer
				calls := 0
				command := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
					if strings.Count(out.String(), "GET ")+strings.Count(out.String(), "POST ") != calls && mode != "write" && mode != "short" && mode != "write-mismatch" {
						t.Fatal("send before output")
					}
					calls++
					if calls == 1 {
						return runner.Response{Status: 200, Body: []byte(tc.raw)}, nil
					}
					if calls == 2 {
						if string(req.Body) != tc.raw {
							t.Fatalf("operational data changed: %s want %s", req.Body, tc.raw)
						}
						if mode == "transport" {
							return runner.Response{}, errors.New(tc.part)
						}
						if mode == "status" || mode == "write-mismatch" {
							return runner.Response{Status: 503}, nil
						}
					}
					return runner.Response{Status: 200}, nil
				}), BuildInfo{})
				command.SetOut(&out)
				command.SetErr(&stderr)
				var failing *failAfterWriter
				if mode == "write" || mode == "write-mismatch" {
					failing = &failAfterWriter{cause: errors.New(tc.part)}
					if mode == "write-mismatch" {
						failing.successes = 1
					}
					command.SetOut(failing)
				}
				if mode == "short" {
					command.SetOut(shortWriter{})
				}
				command.SetArgs([]string{"test", file, "--secret", "secret"})
				code := command.ExecuteContext(context.Background())
				wantCode, wantCalls := 0, 3
				switch mode {
				case "status":
					wantCode, wantCalls = 1, 2
				case "capture", "conflict", "write", "short":
					wantCode, wantCalls = 2, 1
				case "render", "transport", "write-mismatch":
					wantCode, wantCalls = 2, 2
				}
				if code != wantCode || calls != wantCalls {
					t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out.String(), stderr.String())
				}
				combined := out.String() + stderr.String()
				if failing != nil {
					combined += failing.String()
				}
				for _, variant := range template.SecretVariants(tc.raw) {
					if strings.Contains(combined, variant) {
						t.Fatalf("leaked %q: %q", variant, combined)
					}
				}
				if mode == "success" && !strings.Contains(out.String(), "total=3 iniciadas=3 concluídas=3 validadas=2") {
					t.Fatal(out.String())
				}
				if mode == "capture" && (!strings.Contains(stderr.String(), ":5:20:") || strings.Contains(out.String(), "erro de transporte")) {
					t.Fatalf("out=%q err=%q", out.String(), stderr.String())
				}
			})
		}
	}
}

func TestTypedCaptureTextFailuresHaveOriginalPosition(t *testing.T) {
	for _, raw := range []string{`[]`, `{}`} {
		for _, nl := range []string{"\n", "\r\n"} {
			for _, tc := range []struct{ next, position string }{
				{"GET http://example.test/{{x}}", ":5:25:"},
				{"GET http://example.test/next\nX-Value: prefix{{x}}", ":6:16:"},
				{"POST http://example.test/next\n{\n  \"text\": \"prefix{{x}}\"\n}", ":7:18:"},
			} {
				calls := 0
				fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					return runner.Response{Status: 200, Body: []byte(raw)}, nil
				})
				source := strings.ReplaceAll("GET http://example.test/one\nHTTP 200\n[Captures]\nx: jsonpath \"$\"\n"+tc.next+"\nGET http://example.test/last", "\n", nl)
				code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, source))
				if code != 2 || calls != 1 || !strings.Contains(stderr, tc.position+" erro de renderização") || !strings.Contains(out, "concluídas=1 validadas=1") || !strings.Contains(stderr, "valor JSON completo") {
					t.Fatalf("%s: code=%d calls=%d out=%q err=%q", fmt.Sprintf("%q", tc.next), code, calls, out, stderr)
				}
			}
		}
	}
}
