package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func suiteFile(t *testing.T, dir, name, text string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func prevalidationSummary(out string) bool {
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	return len(lines) == 2 && strings.HasPrefix(lines[0], "resumo ") && strings.Contains(lines[0], "total=desconhecido iniciadas=0 concluídas=0") && strings.Contains(lines[0], "resultado=erro") && strings.HasPrefix(lines[1], "resumo suíte: arquivos=1 processados=1 executados=0 sucessos=0 falhas_assertion=0 erros=1")
}

func shownPath(t *testing.T, path string) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	name, err := filepath.Rel(cwd, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(name)
}

func TestSuiteHelpAndArguments(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
		for _, fragment := range []string{"<caminho>...", "--fail-fast", "não executados", "resumo suíte", "Entre unidades", "Não há cookie jar"} {
			if !strings.Contains(out, fragment) {
				t.Fatalf("help missing %q", fragment)
			}
		}
		if strings.Contains(out, "Exatamente um arquivo; parsing") {
			t.Fatal("obsolete single-file restriction")
		}
	}
	for _, flags := range [][]string{{"test", "a", "--fail-fast", "b", "--env=local"}, {"test", "--fail-fast", "--env", "local", "a", "b"}} {
		args, err := parseArguments(flags)
		if err != nil || !args.failFast || !reflect.DeepEqual(args.files, []string{"a", "b"}) || args.environment != "local" {
			t.Fatalf("args=%+v err=%v", args, err)
		}
	}
	for _, flags := range [][]string{{"test"}, {"test", "--fail-fast"}, {"test", "a", "--fail-fast=true"}, {"test", "a", "--fail-fast=false"}, {"test", "a", "--fail-fast="}, {"test", "a", "--unsupported"}} {
		if _, err := parseArguments(flags); err == nil {
			t.Fatalf("accepted %v", flags)
		}
	}
}

func TestSuitePolicyAndIncrementalPresentation(t *testing.T) {
	for _, mode := range []string{"assertion", "transport", "parse", "success"} {
		for _, fast := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", mode, fast), func(t *testing.T) {
				dir := t.TempDir()
				suiteFile(t, dir, "01.hurl", "GET http://example.test/first\nHTTP 200")
				middle := "GET http://example.test/second\nHTTP 200\nGET http://example.test/blocked\nHTTP 200"
				if mode == "parse" {
					middle += "\nHEAD http://example.test/unsupported"
				}
				suiteFile(t, dir, "02.hurl", middle)
				suiteFile(t, dir, "03.hurl", "GET http://example.test/last\nHTTP 200")
				var out, stderr bytes.Buffer
				var calls []string
				command := NewRootCommand(executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
					// The previous file summary and diagnostics must precede the next send.
					if strings.HasSuffix(request.URL, "/second") && !strings.Contains(out.String(), "01.hurl") {
						t.Fatal("missing preceding summary")
					}
					if strings.HasSuffix(request.URL, "/last") && !strings.Contains(out.String(), "02.hurl") {
						t.Fatal("missing second summary")
					}
					calls = append(calls, request.URL)
					if strings.HasSuffix(request.URL, "/second") {
						if mode == "assertion" {
							return runner.Response{Status: 503}, nil
						}
						if mode == "transport" {
							return runner.Response{}, errors.New("private cause")
						}
					}
					return runner.Response{Status: 200}, nil
				}), BuildInfo{})
				args := []string{"test", dir}
				if fast {
					args = append(args, "--fail-fast")
				}
				command.SetArgs(args)
				command.SetOut(&out)
				command.SetErr(&stderr)
				code := command.ExecuteContext(context.Background())
				expected := 2
				if mode == "assertion" {
					expected = 1
				}
				if mode == "success" {
					expected = 0
				}
				if code != expected {
					t.Fatalf("code=%d out=%q err=%q", code, out.String(), stderr.String())
				}
				if fast && mode != "success" {
					if strings.Contains(strings.Join(calls, " "), "/last") || !strings.Contains(out.String(), "resultado=não executado") || !strings.Contains(out.String(), "não_executados=1") {
						t.Fatalf("calls=%v out=%q", calls, out.String())
					}
				} else if calls[len(calls)-1] != "http://example.test/last" {
					t.Fatalf("did not continue: %v", calls)
				}
				if mode == "parse" && (!strings.Contains(out.String(), "total=desconhecido iniciadas=0 concluídas=0") || !strings.Contains(stderr.String(), ":5:1:")) {
					t.Fatalf("out=%q err=%q", out.String(), stderr.String())
				}
			})
		}
	}
}

type suiteWriter struct {
	calls, allowed int
	short          bool
	data           bytes.Buffer
}

func (w *suiteWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls > w.allowed {
		if w.short {
			return len(p) - 1, nil
		}
		return 0, errors.New("private write cause")
	}
	return w.data.Write(p)
}

func TestSuitePresentationFailureStopsAllFiles(t *testing.T) {
	for _, stage := range []string{"entry", "file summary", "diagnostic", "parse diagnostic", "aggregate"} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", stage, short), func(t *testing.T) {
				dir := t.TempDir()
				first := "GET http://example.test/first\nHTTP 200"
				if stage == "parse diagnostic" {
					first += "\nHEAD http://example.test/invalid"
				}
				suiteFile(t, dir, "01.hurl", first)
				suiteFile(t, dir, "02.hurl", "GET http://example.test/last\nHTTP 200")
				calls := 0
				executor := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					status := 200
					if stage == "diagnostic" {
						status = 503
					}
					return runner.Response{Status: status}, nil
				})
				writer := &suiteWriter{short: short}
				if stage == "file summary" {
					writer.allowed = 1
				}
				if stage == "aggregate" {
					writer.allowed = 4
				}
				command := NewRootCommand(executor, BuildInfo{})
				command.SetArgs([]string{"test", dir})
				command.SetOut(io.Discard)
				command.SetErr(io.Discard)
				if strings.Contains(stage, "diagnostic") {
					command.SetErr(writer)
				} else {
					command.SetOut(writer)
				}
				code := command.ExecuteContext(context.Background())
				expectedCalls := 1
				if stage == "aggregate" {
					expectedCalls = 2
				}
				if stage == "parse diagnostic" {
					expectedCalls = 0
				}
				if code != 2 || calls != expectedCalls || writer.calls != writer.allowed+1 {
					t.Fatalf("code=%d calls=%d writer.calls=%d output=%q", code, calls, writer.calls, writer.data.String())
				}
			})
		}
	}
}

func TestSuiteDiscoveryAndConfiguration(t *testing.T) {
	dir := t.TempDir()
	a := suiteFile(t, dir, "01.hurl", "GET {{base}}/a\nHTTP 200")
	b := suiteFile(t, dir, "nested/02.hurl", "GET {{base}}/b\nHTTP 200")
	c := suiteFile(t, dir, "explicit", "GET {{base}}/c\nHTTP 200")
	suiteFile(t, dir, "payload.txt", "ignored")
	for _, paths := range [][]string{{dir, b, a, c}, {c, a, dir, b}} {
		fake := &fakeExecutor{statuses: []int{200, 200, 200}}
		args := append([]string{"test", "--var", "base=http://example.test"}, paths...)
		code, out, stderr := execute(t, context.Background(), fake, args...)
		if code != 0 || stderr != "" || !reflect.DeepEqual(fake.calls, []string{"http://example.test/a", "http://example.test/c", "http://example.test/b"}) || strings.Count(out, "resumo ") != 4 {
			t.Fatalf("code=%d out=%q err=%q calls=%v", code, out, stderr, fake.calls)
		}
	}
	for _, mode := range []string{"missing", "empty", "link", "config", "environment", "reference"} {
		t.Run(mode, func(t *testing.T) {
			fake := &fakeExecutor{}
			args := []string{"test", dir}
			switch mode {
			case "missing":
				args = append(args, filepath.Join(dir, "missing.hurl"))
			case "empty":
				args = append(args, t.TempDir())
			case "link":
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(dir, link); err != nil {
					t.Fatal(err)
				}
				args = append(args, link)
			case "config":
				args = append(args, "--config", suiteFile(t, t.TempDir(), "bad.yaml", "version: 999"))
			case "environment":
				args = append(args, "--env", "../bad")
			case "reference":
				t.Setenv("HTTPGO_SUITE_PRESENT", "fictitious-known-secret")
				t.Setenv("HTTPGO_SUITE_ABSENT", "")
				if err := os.Unsetenv("HTTPGO_SUITE_ABSENT"); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--config", suiteFile(t, t.TempDir(), "base.yaml", "version: 1\nvariables: {known: '${HTTPGO_SUITE_PRESENT}', absent: '${HTTPGO_SUITE_ABSENT}'}"))
			}
			code, out, stderr := execute(t, context.Background(), fake, args...)
			if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 || strings.Contains(stderr, "fictitious-known-secret") {
				t.Fatalf("code=%d out=%q err=%q calls=%v", code, out, stderr, fake.calls)
			}
		})
	}
}
