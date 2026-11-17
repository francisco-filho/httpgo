package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestHTMLFinalProtectionCoversEarlierNamesURLsAndDiagnostics(t *testing.T) {
	const token = "fiction-html-env-<&\""
	const captured = "fiction-html-captured"
	const later = "fiction-html-discovered-later"
	t.Setenv("HTTPGO_HTML_TOKEN", token)
	root := t.TempDir()
	config := suiteFile(t, root, "base.yaml", "version: 1\nvariables:\n  token: ${HTTPGO_HTML_TOKEN}\nsecrets: [captured]")
	dir := filepath.Join(root, "tests")
	suiteFile(t, dir, "01-"+later+".hurl", "GET http://user:private-url-password@example.test/login?token=private-query#private-fragment\nAuthorization: Bearer {{token}}\nHTTP 200\n[Captures]\ncaptured: jsonpath \"$.token\"\nGET http://example.test/"+later+"/{{captured}}\nX-Derived: prefix-{{captured}}\nHTTP 200\n[Asserts]\njsonpath \"$.echo\" == \"different\"")
	suiteFile(t, dir, "02.hurl", "GET http://example.test/discover\nHTTP 200")
	suiteFile(t, dir, "03.hurl", "GET http://example.test/protected\nHTTP 200\n[Asserts]\njsonpath \"$.value\" == {{token}}")
	calls := 0
	fake := executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		calls++
		switch calls {
		case 1:
			if req.URL != "http://user:private-url-password@example.test/login?token=private-query#private-fragment" || req.Headers[0].Value != "Bearer "+token {
				t.Fatal("operational credentials changed")
			}
			return runner.Response{Status: 200, Body: []byte(`{"token":"` + captured + `","private":"BODY_SENTINEL"}`)}, nil
		case 2:
			if !strings.HasSuffix(req.URL, "/"+captured) || req.Headers[0].Value != "prefix-"+captured {
				t.Fatal("capture/derived data changed")
			}
			return runner.Response{Status: 200, Body: []byte(`{"echo":"` + later + `","private":"BODY_SENTINEL"}`)}, nil
		case 3:
			return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: later}, {Name: "X-Private", Value: "HEADER_SENTINEL"}}}, nil
		default:
			body, err := json.Marshal(map[string]string{"value": token + "-different"})
			if err != nil {
				t.Fatal(err)
			}
			return runner.Response{Status: 200, Body: body}, nil
		}
	})
	base := filepath.Join(root, "report.html")
	code, out, stderr := execute(t, context.Background(), fake, "test", dir, "--config", config, "--render-asserts", "--output=html", "--output-file", base)
	_, html := readHTML(t, base)
	if code != 1 || calls != 4 || out != "" || !strings.Contains(html, "[oculto]") || !strings.Contains(html, "esperado") {
		t.Fatalf("code=%d calls=%d out=%q err=%q HTML=%s", code, calls, out, stderr, html)
	}
	for _, secret := range []string{token, captured, "fiction-html-env-", "prefix-" + captured, "private-url-password", "private-query", "private-fragment", "BODY_SENTINEL", "HEADER_SENTINEL"} {
		if strings.Contains(html+stderr, secret) {
			t.Fatalf("leak of %q", secret)
		}
	}
	if strings.Contains(html, later) {
		t.Fatal("late secret survived in HTML metadata/diagnostic")
	}
	// Streaming stderr cannot redact an ordinary value before it is known.
}

func TestHTMLInterruptionsKeepAvailableResultsWithoutRetryingStderr(t *testing.T) {
	for _, mode := range []string{"cancel", "stderr"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			suiteFile(t, dir, "01.hurl", "GET http://example.test/first\nHTTP 200\nGET http://example.test/never\nHTTP 200")
			suiteFile(t, dir, "02.hurl", "GET http://example.test/never\nHTTP 200")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				if mode == "cancel" {
					cancel()
					return runner.Response{}, context.Canceled
				}
				return runner.Response{Status: 503}, nil
			}), BuildInfo{})
			var out, stderr bytes.Buffer
			broken := &htmlCountingWriter{}
			command.SetOut(&out)
			command.SetErr(&stderr)
			if mode == "stderr" {
				command.SetErr(broken)
			}
			base := filepath.Join(t.TempDir(), "report.html")
			command.SetArgs([]string{"test", dir, "--output=html", "--output-file", base})
			code := command.ExecuteContext(ctx)
			_, html := readHTML(t, base)
			if code != 2 || out.Len() != 0 || calls != 1 || !strings.Contains(html, "<strong>não executado</strong>") || !strings.Contains(html, "não iniciadas=1") || !strings.Contains(html, "total=desconhecido") {
				t.Fatalf("code=%d calls=%d out=%q stderr=%q HTML=%s", code, calls, out.String(), stderr.String(), html)
			}
			if mode == "stderr" && broken.calls != 1 {
				t.Fatalf("failed stderr retried %d times", broken.calls)
			}
		})
	}
}

type htmlCountingWriter struct{ calls int }

func (w *htmlCountingWriter) Write([]byte) (int, error) {
	w.calls++
	return 0, fmt.Errorf("private output cause")
}

func TestHTMLCommandReuseDoesNotRetainDataOrDestinations(t *testing.T) {
	const secret = "fiction-html-reuse"
	root := t.TempDir()
	file := suiteFile(t, root, "first.hurl", "GET http://example.test\nHTTP 200\n[Captures]\nid: jsonpath \"$.id\"")
	calls := 0
	command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		return runner.Response{Status: 200, Body: []byte(`{"id":"` + secret + `"}`)}, nil
	}), BuildInfo{})
	var out, stderr bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&stderr)
	first := filepath.Join(root, "first.html")
	command.SetArgs([]string{"test", file, "--secret", "id", "--output=html", "--output-file", first})
	if code := command.ExecuteContext(context.Background()); code != 0 {
		t.Fatal(code, stderr.String())
	}
	firstPath, firstHTML := readHTML(t, first)
	secondFile := suiteFile(t, root, "second.hurl", "GET http://example.test/{{id}}\nHTTP 200")
	second := filepath.Join(root, "second.html")
	command.SetArgs([]string{"test", secondFile, "--output=html", "--output-file", second})
	stderr.Reset()
	if code := command.ExecuteContext(context.Background()); code != 2 || calls != 1 {
		t.Fatalf("code=%d calls=%d stderr=%q", code, calls, stderr.String())
	}
	_, html := readHTML(t, second)
	if strings.Contains(html, "first.hurl") || strings.Contains(stderr.String(), firstPath) || !strings.Contains(html, "erro de renderização") {
		t.Fatal(html, stderr.String())
	}
	// The same value is ordinary in a fresh invocation: redaction must not leak.
	thirdFile := suiteFile(t, root, "third.hurl", "GET http://example.test/"+secret+"\nHTTP 200")
	third := filepath.Join(root, "third.html")
	command.SetArgs([]string{"test", thirdFile, "--output=html", "--output-file", third})
	if code := command.ExecuteContext(context.Background()); code != 0 || calls != 2 {
		t.Fatal(code, calls)
	}
	_, html = readHTML(t, third)
	if !strings.Contains(html, secret) || strings.Contains(html, "erro de renderização") {
		t.Fatal(html)
	}
	_, unchanged := readHTML(t, first)
	if unchanged != firstHTML || command.protection != nil {
		t.Fatal("invocation retained/mutated presentation")
	}
}
