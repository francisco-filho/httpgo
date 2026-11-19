package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
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

type junitDocument struct {
	Tests    int `xml:"tests,attr"`
	Failures int `xml:"failures,attr"`
	Errors   int `xml:"errors,attr"`
	Skipped  int `xml:"skipped,attr"`
	Cases    []struct {
		Name    string  `xml:"name,attr"`
		Time    float64 `xml:"time,attr"`
		Failure *string `xml:"failure"`
		Error   *string `xml:"error"`
		Skipped *string `xml:"skipped"`
	} `xml:"testcase"`
}

func readJUnit(t *testing.T, path string) (junitDocument, string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc junitDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("XML: %v: %s", err, data)
	}
	return doc, string(data)
}

func TestJUnitArgumentsAndHelp(t *testing.T) {
	for _, args := range [][]string{
		{"test", "a", "--output", "junit", "--output-file", "out.xml"},
		{"test", "--output-file=out.xml", "a", "--output=junit"},
	} {
		got, err := parseArguments(args)
		if err != nil || got.output != "junit" || got.outputFile != "out.xml" || len(got.files) != 1 {
			t.Fatalf("args=%v got=%+v err=%v", args, got, err)
		}
	}
	for _, flags := range [][]string{
		{"--output"}, {"--output-file"}, {"--output=junit"}, {"--output-file=out.xml"},
		{"--output=json", "--output-file=out.xml"}, {"--output=", "--output-file=out.xml"},
		{"--output=junit", "--output-file="},
		{"--output=junit", "--output=junit", "--output-file=out.xml"},
		{"--output=junit", "--output-file=out.xml", "--output-file=out.xml"},
	} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, append([]string{"test", "a"}, flags...)...)
		if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 {
			t.Fatalf("flags=%v code=%d out=%q stderr=%q calls=%v", flags, code, out, stderr, fake.calls)
		}
	}
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, args...)
		if code != 0 || stderr != "" || !strings.Contains(out, "--output junit --output-file target/results.xml") || !strings.Contains(out, "exige --output-file") {
			t.Fatalf("help code=%d out=%q stderr=%q", code, out, stderr)
		}
	}
}

func TestJUnitSuiteClassificationAndNoReexecution(t *testing.T) {
	for _, mode := range []string{"success", "assertion", "transport", "parse", "capture", "render"} {
		for _, fast := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", mode, fast), func(t *testing.T) {
				dir := t.TempDir()
				suiteFile(t, dir, "01-ação & aspas.hurl", "GET http://example.test/ok\nHTTP 200")
				middle := "GET http://example.test/middle\nHTTP 200"
				switch mode {
				case "parse":
					middle += "\nHEAD http://example.test/unsupported"
				case "capture":
					middle += "\n[Captures]\nid: jsonpath \"$.absent\""
				case "render":
					middle = "GET http://example.test/{{missing}}"
				}
				suiteFile(t, dir, "02.hurl", middle)
				suiteFile(t, dir, "03.hurl", "GET http://example.test/last\nHTTP 200")
				calls := 0
				fake := executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
					calls++
					if strings.HasSuffix(req.URL, "/middle") {
						if mode == "assertion" {
							return runner.Response{Status: 503}, nil
						}
						if mode == "transport" {
							return runner.Response{}, errors.New("private transport cause")
						}
					}
					return runner.Response{Status: 200, Body: []byte(`{}`)}, nil
				})
				path := filepath.Join(t.TempDir(), "new", "nested", "results.xml")
				args := []string{"test", dir, "--output=junit", "--output-file", path}
				if fast {
					args = append(args, "--fail-fast")
				}
				code, out, stderr := execute(t, context.Background(), fake, args...)
				doc, raw := readJUnit(t, path)
				wantCode, wantCalls := 2, 3
				if mode == "parse" || mode == "render" {
					wantCalls--
				}
				if mode == "success" {
					wantCode = 0
				} else {
					if fast {
						wantCalls--
					}
					if mode == "assertion" {
						wantCode = 1
					}
				}
				if code != wantCode || calls != wantCalls || out != "" || doc.Tests != 3 || len(doc.Cases) != 3 || strings.Contains(raw+stderr, "private transport cause") {
					t.Fatalf("code=%d calls=%d out=%q stderr=%q XML=%s", code, calls, out, stderr, raw)
				}
				if mode == "success" {
					if doc.Failures+doc.Errors+doc.Skipped != 0 || stderr != "" {
						t.Fatalf("success: %+v stderr=%q", doc, stderr)
					}
				} else {
					if doc.Failures+doc.Errors != 1 || stderr == "" {
						t.Fatalf("failure: %+v stderr=%q XML=%s", doc, stderr, raw)
					}
					if (mode == "assertion") != (doc.Cases[1].Failure != nil) || (fast != (doc.Cases[2].Skipped != nil)) {
						t.Fatalf("classification: %+v", doc)
					}
				}
			})
		}
	}
}

func TestJUnitPreexecutionErrorsLeaveDestinationUntouched(t *testing.T) {
	for _, mode := range []string{"config", "discovery", "cancelled"} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", mode, existing), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "out.xml")
				if existing {
					if err := os.WriteFile(path, []byte("unchanged"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				file := writeScenario(t, "GET http://example.test")
				args := []string{"test", file, "--output=junit", "--output-file", path}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				switch mode {
				case "config":
					args = append(args, "--config", suiteFile(t, t.TempDir(), "bad.yaml", "version: 999"))
				case "discovery":
					args = append(args, filepath.Join(t.TempDir(), "missing"))
				case "cancelled":
					cancel()
				}
				fake := &fakeExecutor{}
				code, out, stderr := execute(t, ctx, fake, args...)
				data, err := os.ReadFile(path)
				if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 || (existing && (err != nil || string(data) != "unchanged")) || (!existing && !errors.Is(err, os.ErrNotExist)) {
					t.Fatalf("code=%d out=%q stderr=%q data=%q err=%v", code, out, stderr, data, err)
				}
			})
		}
	}
}

type junitWriter struct {
	bytes.Buffer
	short, fail, closed bool
	closeErr            error
}

func (w *junitWriter) Write(p []byte) (int, error) {
	if w.fail {
		return 0, errors.New("private write cause")
	}
	if w.short {
		return len(p) - 1, nil
	}
	return w.Buffer.Write(p)
}
func (w *junitWriter) Close() error { w.closed = true; return w.closeErr }

func TestJUnitDestinationFailures(t *testing.T) {
	for _, mode := range []string{"directory", "parent file", "invalid", "open", "write", "short", "close"} {
		t.Run(mode, func(t *testing.T) {
			file := writeScenario(t, "GET http://example.test\nHTTP 200")
			path := filepath.Join(t.TempDir(), "out.xml")
			fake := &fakeExecutor{statuses: []int{503}}
			command := NewRootCommand(fake, BuildInfo{})
			writer := &junitWriter{}
			switch mode {
			case "directory":
				path = t.TempDir()
			case "parent file":
				path = filepath.Join(file, "out.xml")
			case "invalid":
				path += "\x00"
			default:
				command.openReport = func(string) (io.WriteCloser, error) {
					if mode == "open" {
						return nil, errors.New("private open cause")
					}
					return writer, nil
				}
				writer.fail, writer.short = mode == "write", mode == "short"
				if mode == "close" {
					writer.closeErr = errors.New("private close cause")
				}
			}
			var out, stderr bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&stderr)
			command.SetArgs([]string{"test", file, "--output=junit", "--output-file", path})
			code := command.ExecuteContext(context.Background())
			if code != 2 || out.Len() != 0 || len(fake.calls) != 1 || !strings.Contains(stderr.String(), "não foi possível gravar relatório JUnit") || strings.Contains(stderr.String(), "private") || strings.Contains(stderr.String(), "publicado") {
				t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), stderr.String())
			}
			if (mode == "write" || mode == "short" || mode == "close") && !writer.closed {
				t.Fatal("report writer was not closed")
			}
		})
	}
}

func TestJUnitSecretExpectedAndActualOperands(t *testing.T) {
	const secret = "fiction-operand-junit-<&\""
	t.Setenv("HTTPGO_JUNIT_OPERAND", secret)
	root := t.TempDir()
	config := suiteFile(t, root, "base.yaml", "version: 1\nvariables:\n  protected: ${HTTPGO_JUNIT_OPERAND}")
	file := suiteFile(t, root, "input.hurl", "GET http://example.test\nHTTP 200\n[Asserts]\njsonpath \"$.value\" == {{protected}}")
	body, err := json.Marshal(map[string]string{"value": secret + "-different"})
	if err != nil {
		t.Fatal(err)
	}
	fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		return runner.Response{Status: 200, Body: body}, nil
	})
	path := filepath.Join(root, "out.xml")
	code, out, stderr := execute(t, context.Background(), fake, "test", file, "--config", config, "--render-asserts", "--output=junit", "--output-file", path)
	doc, raw := readJUnit(t, path)
	if code != 1 || out != "" || doc.Failures != 1 || strings.Contains(raw+stderr, "fiction-operand-junit") || !strings.Contains(*doc.Cases[0].Failure, "esperado") || strings.Count(*doc.Cases[0].Failure, "[oculto]") < 2 {
		t.Fatalf("code=%d out=%q stderr=%q XML=%s", code, out, stderr, raw)
	}
}

func TestJUnitDiagnosticFailureAndCancellationStillReport(t *testing.T) {
	for _, mode := range []string{"stderr", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			suiteFile(t, dir, "01.hurl", "GET http://example.test/first\nHTTP 200")
			suiteFile(t, dir, "02.hurl", "GET http://example.test/not-sent\nHTTP 200")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				if mode == "cancelled" {
					cancel()
					return runner.Response{}, context.Canceled
				}
				return runner.Response{Status: 503}, nil
			}), BuildInfo{})
			var out bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&suiteWriter{short: true})
			if mode == "cancelled" {
				command.SetErr(io.Discard)
			}
			path := filepath.Join(t.TempDir(), "out.xml")
			command.SetArgs([]string{"test", dir, "--output=junit", "--output-file", path})
			code := command.ExecuteContext(ctx)
			doc, raw := readJUnit(t, path)
			if code != 2 || out.Len() != 0 || calls != 1 || doc.Skipped != 1 || doc.Tests != 2 || doc.Errors+doc.Failures != 1 {
				t.Fatalf("code=%d calls=%d out=%q XML=%s", code, calls, out.String(), raw)
			}
		})
	}
}

func TestJUnitProtectsEarlierDiagnosticsAndNames(t *testing.T) {
	const secret = "fiction-junit-secret<&\""
	t.Setenv("HTTPGO_JUNIT_SECRET", secret)
	root := t.TempDir()
	config := suiteFile(t, root, "config.yaml", "version: 1\nvariables:\n  token: ${HTTPGO_JUNIT_SECRET}")
	dir := filepath.Join(root, "tests")
	suiteFile(t, dir, "01.hurl", "GET http://example.test/first\nAuthorization: Bearer {{token}}\nHTTP 200\n[Asserts]\njsonpath \"$.echo\" == \"different\"")
	// A later sensitive header discovers a value previously shown as ordinary.
	suiteFile(t, dir, "02.hurl", "GET http://example.test/login\nHTTP 200")
	const laterSecret = "fiction-later-junit-secret"
	suiteFile(t, dir, "03-"+laterSecret+".hurl", "GET http://example.test/echo\nHTTP 200\n[Asserts]\nheader \"X-Echo\" == \"different\"")
	calls := 0
	fake := executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		calls++
		if calls == 1 {
			if req.URL != "http://example.test/first" || req.Headers[0].Value != "Bearer "+secret {
				t.Fatal("operational secret was changed")
			}
			return runner.Response{Status: 200, Body: []byte(`{"echo":"fiction-later-junit-secret"}`)}, nil
		}
		return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: laterSecret}, {Name: "X-Echo", Value: secret}}}, nil
	})
	path := filepath.Join(root, "out.xml")
	code, out, stderr := execute(t, context.Background(), fake, "test", dir, "--config", config, "--output=junit", "--output-file", path)
	doc, raw := readJUnit(t, path)
	// The artifact is sanitized again after every file has finished. Streaming
	// stderr cannot retroactively redact a value not yet identified as secret.
	if code != 1 || calls != 3 || out != "" || doc.Failures != 2 || strings.Contains(raw, secret) || strings.Contains(raw, laterSecret) || strings.Contains(stderr, secret) || !strings.Contains(raw, "[oculto]") {
		t.Fatalf("code=%d calls=%d out=%q stderr=%q XML=%s", code, calls, out, stderr, raw)
	}
}
