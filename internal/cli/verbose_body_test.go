package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func verboseResponse(req runner.Request, status int, body string) runner.Response {
	state := runner.BodyJSON
	if req.Body == nil {
		state = runner.BodyAbsent
	}
	return runner.Response{Status: status, Body: []byte(body), Diagnostics: &runner.HTTPDiagnostics{Request: runner.HTTPBody{State: state, Bytes: req.Body}, Response: runner.HTTPBody{State: runner.BodyJSON, Bytes: []byte(body)}}}
}

func TestVerboseBodySelectionEffectivePayloads(t *testing.T) {
	for _, mode := range []string{"inline", "inline rendered", "external", "external rendered", "external literal", "empty", "binary", "absent", "unavailable", "success", "level1"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			payload := `{"payload":"{{value}}"}`
			source := "POST http://example.test\n" + payload + "\nHTTP 200"
			flags := []string{"--var", "value=rendered", "-vv"}
			expected := `{"payload":"rendered"}`
			switch mode {
			case "inline":
				source = "POST http://example.test\n{\"payload\":1}\nHTTP 200"
				expected = `{"payload":1}`
			case "external", "external rendered", "external literal", "empty", "binary":
				source = "POST http://example.test\nfile,body.json;\nHTTP 200"
				if mode == "external rendered" {
					flags = append(flags, "--render-files")
				} else {
					expected = payload
				}
				if mode == "external" {
					payload = `{"external":true}`
					expected = payload
				}
				if mode == "empty" {
					payload = ""
				}
				if mode == "binary" {
					payload = "private\x00binary"
				}
				if err := os.WriteFile(dir+"/body.json", []byte(payload), 0600); err != nil {
					t.Fatal(err)
				}
			case "absent":
				source = "POST http://example.test\nHTTP 200"
			case "level1":
				flags = []string{"--var", "value=rendered", "-v"}
			}
			file := suiteFile(t, dir, "one.hurl", source)
			var out, stderr bytes.Buffer
			calls := 0
			c := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
				calls++
				if req.DiagnosticBodies != (mode != "level1") {
					t.Fatalf("diagnostic selection=%t", req.DiagnosticBodies)
				}
				if mode != "empty" && mode != "binary" && mode != "absent" && string(req.Body) != expected {
					t.Fatalf("sent=%q want=%q", req.Body, expected)
				}
				status := 503
				if mode == "success" {
					status = 200
				}
				response := verboseResponse(req, status, `{"response":"final"}`)
				if mode == "empty" {
					response.Diagnostics.Request.State = runner.BodyJSON
				}
				if mode == "unavailable" {
					response.Diagnostics = nil
				}
				return response, nil
			}), BuildInfo{})
			c.SetOut(&out)
			c.SetErr(&stderr)
			c.SetArgs(append([]string{"test", file}, flags...))
			code := c.ExecuteContext(context.Background())
			want := 1
			if mode == "success" {
				want = 0
			}
			if code != want || calls != 1 {
				t.Fatalf("code=%d sends=%d out=%q err=%q", code, calls, out.String(), stderr.String())
			}
			text := stderr.String()
			if strings.Contains(out.String(), "body") || strings.Contains(text, "\r") || strings.Contains(text, "\x00") {
				t.Fatalf("unsafe streams: %q %q", out.String(), text)
			}
			if mode == "success" || mode == "level1" {
				if strings.Contains(text, "[request body]") || strings.Contains(text, "[response body]") {
					t.Fatalf("unexpected bodies: %q", text)
				}
				return
			}
			if strings.Count(text, "[request body]") != 1 || strings.Count(text, "[response body]") != 1 {
				t.Fatalf("missing blocks: %q", text)
			}
			switch mode {
			case "empty", "binary":
				if !strings.Contains(text, "omitido (conteúdo não JSON)") || strings.Contains(text, "private") {
					t.Fatalf("not omitted: %q", text)
				}
			case "absent":
				if !strings.Contains(text, "[request body]\n  ausente") {
					t.Fatalf("not absent: %q", text)
				}
			case "unavailable":
				if strings.Count(text, "indisponível/incompleto") != 2 {
					t.Fatalf("not unavailable: %q", text)
				}
			default:
				if !strings.Contains(text, expected) || !strings.Contains(text, `{"response":"final"}`) {
					t.Fatalf("wrong preview: %q", text)
				}
			}
		})
	}
}

func TestVerboseBodyWriterFailures(t *testing.T) {
	for _, short := range []bool{false, true} {
		for allowed := 0; allowed < 6; allowed++ {
			t.Run(fmt.Sprintf("short=%t/allowed=%d", short, allowed), func(t *testing.T) {
				dir := t.TempDir()
				suiteFile(t, dir, "01.hurl", "POST http://example.test\n{}\nHTTP 200")
				suiteFile(t, dir, "02.hurl", "GET http://example.test/blocked")
				calls := 0
				c := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
					calls++
					return verboseResponse(req, 503, `{}`), nil
				}), BuildInfo{})
				var out bytes.Buffer
				stderr := &suiteWriter{allowed: allowed, short: short}
				c.SetOut(&out)
				c.SetErr(stderr)
				c.SetArgs([]string{"test", dir, "-vv"})
				code := c.ExecuteContext(context.Background())
				if code != 2 || calls > 1 || stderr.calls != allowed+1 || out.Len() != 0 {
					t.Fatalf("code=%d sends=%d writes=%d stdout=%q", code, calls, stderr.calls, out.String())
				}
			})
		}
	}
	file := suiteFile(t, t.TempDir(), "one.hurl", "GET http://example.test")
	c := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		return verboseResponse(req, 200, `{}`), nil
	}), BuildInfo{})
	stdout := &suiteWriter{allowed: 0, short: true}
	var stderr bytes.Buffer
	c.SetOut(stdout)
	c.SetErr(&stderr)
	c.SetArgs([]string{"test", file, "-vv"})
	if code := c.ExecuteContext(context.Background()); code != 2 || stdout.calls != 1 {
		t.Fatalf("code=%d stdout writes=%d", code, stdout.calls)
	}
}

func TestVerboseBodyInvocationProtectionAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	suiteFile(t, dir, "01.hurl", "GET http://example.test/login\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"")
	suiteFile(t, dir, "02.hurl", "POST http://example.test/fiction-private-known?key=private-query\n{\"token\":\"fiction-private-known\"}\nHTTP 200")
	var out, stderr bytes.Buffer
	calls := 0
	c := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		calls++
		if calls == 1 {
			return verboseResponse(req, 200, `{"token":"fiction-private-known"}`), nil
		}
		if !strings.Contains(req.URL, "fiction-private-known") || !strings.Contains(string(req.Body), "fiction-private-known") {
			t.Fatal("operational data sanitized")
		}
		return verboseResponse(req, 503, `{"token":"fiction-private-known"}`), nil
	}), BuildInfo{})
	c.SetOut(&out)
	c.SetErr(&stderr)
	c.SetArgs([]string{"test", dir, "-vv", "--secret", "token"})
	if code := c.ExecuteContext(context.Background()); code != 1 || calls != 2 {
		t.Fatalf("code=%d calls=%d err=%q", code, calls, stderr.String())
	}
	text := out.String() + stderr.String()
	if strings.Contains(text, "fiction-private-known") || strings.Contains(text, "private-query") || strings.Count(text, "[request body]") != 1 || strings.Count(text, "[oculto]") < 3 {
		t.Fatalf("unsafe output %q", text)
	}
}
