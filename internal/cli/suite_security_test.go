package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/suite"
	"capimgrosso.com/httpgo/internal/template"
)

func TestSuiteProtectionSurvivesSessionChangesBeforeCollection(t *testing.T) {
	const token = "fiction-suite-capture"
	const cookie = "fiction-suite-cookie"
	dir := t.TempDir()
	suiteFile(t, dir, "01.hurl", "GET http://example.test/login\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\nPOST http://example.test/use\nAuthorization: Bearer {{token}}\n[MultipartFormData]\ntext: prefix-{{token}}\nfile: file,payload.bin;")
	suiteFile(t, dir, "payload.bin", "literal")
	suiteFile(t, dir, "02.hurl", "GET http://example.test/neutral\nHTTP 200")
	suiteFile(t, dir, "03-"+token+".hurl", "GET http://example.test/echo\nHTTP 200\n[Asserts]\nheader \"X-Echo\" == \"different\"")
	suiteFile(t, dir, "04-"+cookie+".hurl", "GET http://example.test/first\nHEAD http://example.test/invalid")
	suiteFile(t, dir, "05.hurl", "GET http://example.test/{{token}}")
	contextValues, err := template.New(nil, []string{"token"})
	if err != nil {
		t.Fatal(err)
	}
	protection := &executionProtection{initial: func(text string) string { return text }}
	var calls []string
	execution := runner.NewWithRenderer(executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
		calls = append(calls, request.URL)
		switch {
		case strings.HasSuffix(request.URL, "/login"):
			return runner.Response{Status: 200, Body: []byte(`{"token":"` + token + `"}`), Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: cookie}}}, nil
		case strings.HasSuffix(request.URL, "/use"):
			if request.Headers[0].Value != "Bearer "+token || request.Multipart.Parts[0].Value != "prefix-"+token {
				t.Fatal("operational value sanitized")
			}
		case strings.HasSuffix(request.URL, "/echo"):
			return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "X-Echo", Value: "prefix-" + token + " " + cookie}}}, nil
		}
		return runner.Response{Status: 200}, nil
	}), sessionRenderer{contextValues, protection}, func(entry runner.EntryResult) error {
		encoded, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if strings.Contains(string(encoded), token) || strings.Contains(string(encoded), cookie) {
			t.Fatalf("callback leaked: %s", encoded)
		}
		return nil
	})
	s := suite.NewWithProtection(execution, protection.sanitize)
	files, err := s.Discover(context.Background(), []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	result := s.Run(context.Background(), files, suite.Options{}, func(file suite.FileResult, run runner.Result, loadErr error) error {
		text := fmt.Sprintf("%+v %+v", file, run.ErrorPosition)
		if loadErr != nil {
			text += loadErr.Error()
		}
		if run.Err != nil {
			text += run.Err.Error()
		}
		if strings.Contains(text, token) || strings.Contains(text, cookie) {
			t.Fatalf("file callback leaked: %s", text)
		}
		return nil
	})
	encoded, err := json.Marshal(result.Files)
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 2 || len(calls) != 4 || strings.Contains(string(encoded), token) || strings.Contains(string(encoded), cookie) || result.Files[4].Requests.Started != 0 {
		t.Fatalf("result=%s code=%d calls=%v", encoded, result.Code, calls)
	}
	if !strings.Contains(protection.sanitize("prefix-"+token), "[oculto]") {
		t.Fatal("lost derived protection")
	}
}

func TestSuiteSecretsOnStreamsAndLateErrors(t *testing.T) {
	const token = "fiction-invocation-secret"
	const cookie = "fiction-cookie-secret"
	for _, fast := range []bool{false, true} {
		t.Run(fmt.Sprint(fast), func(t *testing.T) {
			dir := t.TempDir()
			suiteFile(t, dir, "01.hurl", "GET http://example.test/login\n[Captures]\ntoken: jsonpath \"$.token\"")
			suiteFile(t, dir, "02.hurl", "GET http://example.test/neutral")
			suiteFile(t, dir, "03-"+token+".hurl", "GET http://example.test/echo\nHTTP 200\n[Asserts]\njsonpath \"$.echo\" == \"different\"")
			suiteFile(t, dir, "04-"+cookie+".hurl", "GET http://example.test/never\nHEAD http://example.test/invalid")
			fake := executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
				if strings.HasSuffix(request.URL, "/login") {
					return runner.Response{Status: 200, Body: []byte(`{"token":"` + token + `"}`), Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: cookie}}}, nil
				}
				if strings.HasSuffix(request.URL, "/echo") {
					return runner.Response{Status: 200, Body: []byte(`{"echo":"` + token + ` ` + cookie + `"}`)}, nil
				}
				return runner.Response{Status: 200}, nil
			})
			args := []string{"test", dir, "--secret", "token"}
			if fast {
				args = append(args, "--fail-fast")
			}
			code, out, stderr := execute(t, context.Background(), fake, args...)
			expected := 2
			if fast {
				expected = 1
			}
			if code != expected || strings.Contains(out+stderr, token) || strings.Contains(out+stderr, cookie) || !strings.Contains(out, "[oculto]") {
				t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
			}
		})
	}
}

func TestSuiteInvocationReuseHasNoStateOrOptionLeak(t *testing.T) {
	dir := t.TempDir()
	suiteFile(t, dir, "01.hurl", "GET http://example.test/login\n[Captures]\ntoken: jsonpath \"$.token\"")
	suiteFile(t, dir, "02.hurl", "POST http://example.test/payload\nfile,payload.json;")
	suiteFile(t, dir, "payload.json", `{"value":{{value}}}`)
	var out, stderr bytes.Buffer
	var requests []runner.Request
	command := NewRootCommand(executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
		requests = append(requests, request)
		if strings.HasSuffix(request.URL, "/login") {
			return runner.Response{Status: 200, Body: []byte(`{"token":"fiction-reuse-secret"}`)}, nil
		}
		return runner.Response{Status: 200}, nil
	}), BuildInfo{})
	command.SetOut(&out)
	command.SetErr(&stderr)
	command.SetArgs([]string{"test", dir, "--render-files", "--var", "value=123", "--secret", "token", "--fail-fast"})
	if code := command.ExecuteContext(context.Background()); code != 0 || string(requests[1].Body) != `{"value":123}` {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	requests = nil
	fresh := suiteFile(t, t.TempDir(), "fresh.hurl", "GET http://example.test/fiction-reuse-secret\nPOST http://example.test/payload\nfile,"+filepath.Join(dir, "payload.json")+";")
	command.SetArgs([]string{"test", fresh})
	if code := command.ExecuteContext(context.Background()); code != 0 || len(requests) != 2 || string(requests[1].Body) != `{"value":{{value}}}` || !strings.Contains(out.String(), "fiction-reuse-secret") {
		t.Fatalf("code=%d requests=%+v out=%q err=%q", code, requests, out.String(), stderr.String())
	}
	out.Reset()
	stderr.Reset()
	requests = nil
	missing := suiteFile(t, t.TempDir(), "missing.hurl", "GET http://example.test/{{token}}")
	command.SetArgs([]string{"test", missing})
	if code := command.ExecuteContext(context.Background()); code != 2 || len(requests) != 0 || !strings.Contains(stderr.String(), "variável ausente") {
		t.Fatalf("code=%d requests=%v err=%q", code, requests, stderr.String())
	}
}

func TestSuiteLateSecretPresentationFailure(t *testing.T) {
	const token = "fiction-late-writer-secret"
	for _, stream := range []string{"stdout", "stderr"} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", stream, short), func(t *testing.T) {
				dir := t.TempDir()
				suiteFile(t, dir, "01.hurl", "GET http://example.test/login\n[Captures]\ntoken: jsonpath \"$.token\"")
				suiteFile(t, dir, "02.hurl", "GET http://example.test/neutral")
				suiteFile(t, dir, "03-"+token+".hurl", "GET http://example.test/not-sent\nHEAD http://example.test/invalid")
				suiteFile(t, dir, "04.hurl", "GET http://example.test/never")
				calls := 0
				command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					return runner.Response{Status: 200, Body: []byte(`{"token":"` + token + `"}`)}, nil
				}), BuildInfo{})
				writer := &suiteWriter{short: short}
				var other bytes.Buffer
				command.SetOut(&other)
				command.SetErr(writer)
				if stream == "stdout" {
					writer.allowed = 4
					command.SetOut(writer)
					command.SetErr(&other)
				}
				command.SetArgs([]string{"test", dir, "--secret", "token"})
				code := command.ExecuteContext(context.Background())
				if code != 2 || calls != 2 || writer.calls != writer.allowed+1 || strings.Contains(writer.data.String()+other.String(), token) {
					t.Fatalf("code=%d calls=%d writes=%d out=%q other=%q", code, calls, writer.calls, writer.data.String(), other.String())
				}
			})
		}
	}
}

func TestSuiteInitialOptionsAndPayloadOriginsRemainIndependent(t *testing.T) {
	root := t.TempDir()
	var files []string
	for _, name := range []string{"a", "b"} {
		dir := filepath.Join(root, name)
		files = append(files, suiteFile(t, dir, "input.hurl", "POST {{base}}/"+name+"\nfile,payload.json;"))
		suiteFile(t, dir, "payload.json", `{"from":"`+name+`","value":{{value}}}`)
	}
	config := suiteFile(t, t.TempDir(), "base.yaml", "version: 1\ndefaults: {timeout: 1.234}\nvariables: {base: http://example.test, value: 9007199254740993}")
	calls := 0
	fake := executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
		name := []string{"a", "b"}[calls]
		calls++
		if request.Timeout.Milliseconds() != 1234 || string(request.Body) != `{"from":"`+name+`","value":9007199254740993}` {
			t.Fatalf("request=%+v", request)
		}
		return runner.Response{Status: 200}, nil
	})
	args := append([]string{"test", "--config", config, "--render-files"}, files...)
	code, out, stderr := execute(t, context.Background(), fake, args...)
	if code != 0 || calls != 2 || stderr != "" {
		t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out, stderr)
	}
	// The suite never changes process working directory.
	if _, err := os.Stat(files[0]); err != nil {
		t.Fatal(err)
	}
}
