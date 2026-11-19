package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestVerboseArguments(t *testing.T) {
	for _, flags := range [][]string{{"-v"}, {"--verbose"}, {"-vv"}, {"-v", "-v"}, {"--verbose", "--verbose"}, {"-v", "--verbose"}, {"--verbose", "-v"}} {
		args, err := parseArguments(append([]string{"test", "file.hurl"}, flags...))
		want := 2
		if len(flags) == 1 && flags[0] != "-vv" {
			want = 1
		}
		if err != nil || args.verbosity != want {
			t.Fatalf("flags=%v level=%d err=%v", flags, args.verbosity, err)
		}
	}
	for _, flags := range [][]string{{"-vv", "-v"}, {"--verbose", "--verbose", "--verbose"}, {"-vvv"}, {"--verbose=2"}, {"-v=false"}} {
		if _, err := parseArguments(append([]string{"test", "file.hurl"}, flags...)); err == nil {
			t.Fatalf("accepted %v", flags)
		}
	}
}

func TestVerboseReportsRejectedBeforeEffects(t *testing.T) {
	for _, format := range []string{"junit", "html"} {
		for _, existing := range []bool{false, true} {
			for _, first := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%t/%t", format, existing, first), func(t *testing.T) {
					dest := t.TempDir() + "/report"
					if existing {
						if err := os.WriteFile(dest, []byte("original"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					file := suiteFile(t, t.TempDir(), "one.hurl", "GET http://example.test")
					flags := []string{"--output", format, "--output-file", dest}
					if first {
						flags = append([]string{"-v"}, flags...)
					} else {
						flags = append(flags, "-v")
					}
					calls := 0
					command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
						calls++
						return runner.Response{Status: 200}, nil
					}), BuildInfo{})
					var out, stderr bytes.Buffer
					command.SetOut(&out)
					command.SetErr(&stderr)
					command.SetArgs(append([]string{"test", file}, flags...))
					if code := command.ExecuteContext(context.Background()); code != 2 || calls != 0 || out.Len() != 0 {
						t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out.String(), stderr.String())
					}
					data, err := os.ReadFile(dest)
					if existing && (err != nil || string(data) != "original") || !existing && !os.IsNotExist(err) {
						t.Fatalf("destination changed: %q %v", data, err)
					}
				})
			}
		}
	}
}

func TestVerboseLifecycleAndDiagnostics(t *testing.T) {
	for _, mode := range []string{"pass", "status", "assertion", "capture", "render", "transport", "parse", "retry", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			source := "GET http://example.test/one\nHTTP 200\nGET http://example.test/two\nHTTP *"
			want, phase := 0, ""
			switch mode {
			case "status":
				want, phase = 1, "status"
			case "assertion":
				source = "GET http://example.test/one\nHTTP 200\n[Asserts]\njsonpath \"$.items\" contains \"SOY\""
				want, phase = 1, "assertion"
			case "capture":
				source = "GET http://example.test/one\n[Captures]\nx: jsonpath \"$.missing\""
				want, phase = 2, "captures"
			case "render":
				source = "GET http://example.test/{{missing}}"
				want, phase = 2, "renderização"
			case "transport":
				want, phase = 2, "transporte"
			case "parse":
				source += "\n[Unsupported]"
				want, phase = 2, "parsing"
			case "retry":
				source = "GET http://example.test/one\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200"
			case "cancel":
				want, phase = 2, "transporte"
			}
			suiteFile(t, dir, "a/same.hurl", source)
			suiteFile(t, dir, "b/same.hurl", "GET http://example.test/last")
			var out, stderr bytes.Buffer
			calls := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			command := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
				calls++
				if !strings.Contains(stderr.String(), "same.hurl\n") {
					t.Fatal("file not announced before HTTP")
				}
				if mode == "transport" && calls == 1 {
					return runner.Response{}, errors.New("private-raw-cause")
				}
				if mode == "cancel" {
					cancel()
					return runner.Response{}, ctx.Err()
				}
				status := 200
				if mode == "status" || mode == "retry" && calls == 1 {
					status = 503
				}
				if strings.HasSuffix(req.URL, "/last") && phase != "" && !strings.Contains(stderr.String(), "["+phase+"]") {
					t.Fatalf("diagnostic not before next file: %q", stderr.String())
				}
				return runner.Response{Status: status, Body: []byte(`{"items":["MILK"]}`)}, nil
			}), BuildInfo{})
			command.SetOut(&out)
			command.SetErr(&stderr)
			command.SetArgs([]string{"test", dir, "-v"})
			if code := command.ExecuteContext(ctx); code != want {
				t.Fatalf("code=%d want=%d err=%q", code, want, stderr.String())
			}
			text := stderr.String()
			if strings.Contains(text, "\r") || strings.Contains(text, "\x1b") || strings.Contains(text, "private-raw-cause") || strings.Contains(text, "[request body]") || strings.Contains(text, "a/same") {
				t.Fatalf("invalid verbose output %q", text)
			}
			if strings.Contains(out.String(), "GET") || !strings.Contains(out.String(), "Testes executados:") {
				t.Fatalf("stdout=%q", out.String())
			}
			if phase != "" && strings.Count(text, "["+phase+"]") != 1 {
				t.Fatalf("phase %s not unique: %q", phase, text)
			}
			if mode == "parse" && strings.Contains(text, "step ") {
				t.Fatalf("invented step: %q", text)
			}
			if mode == "retry" && (calls != 3 || strings.Count(text, "[OK]") != 2 || strings.Contains(text, "[ERRO]")) {
				t.Fatalf("retry calls=%d output=%q", calls, text)
			}
			if mode == "cancel" && !strings.Contains(text, "NÃO EXECUTADO") {
				t.Fatalf("missing skipped %q", text)
			}
		})
	}
}

func TestVerboseWritersAndFailFast(t *testing.T) {
	for _, short := range []bool{false, true} {
		for allowed := 0; allowed < 4; allowed++ {
			t.Run(fmt.Sprintf("%t/%d", short, allowed), func(t *testing.T) {
				dir := t.TempDir()
				suiteFile(t, dir, "01.hurl", "GET http://example.test\nHTTP 200")
				suiteFile(t, dir, "02.hurl", "GET http://example.test/blocked")
				calls := 0
				c := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					return runner.Response{Status: 503}, nil
				}), BuildInfo{})
				writer := &suiteWriter{allowed: allowed, short: short}
				var out bytes.Buffer
				c.SetOut(&out)
				c.SetErr(writer)
				c.SetArgs([]string{"test", dir, "-v"})
				if code := c.ExecuteContext(context.Background()); code != 2 || writer.calls != allowed+1 || calls > 1 {
					t.Fatalf("code=%d writes=%d sends=%d", code, writer.calls, calls)
				}
				if allowed == 0 && calls != 0 {
					t.Fatal("sent after failed announcement")
				}
			})
		}
	}
	dir := t.TempDir()
	suiteFile(t, dir, "01.hurl", "GET http://example.test\nHTTP 200")
	suiteFile(t, dir, "02.hurl", "invalid syntax not loaded")
	c := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		return runner.Response{Status: 503}, nil
	}), BuildInfo{})
	var out, stderr bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&stderr)
	c.SetArgs([]string{"test", dir, "-v", "--fail-fast"})
	if code := c.ExecuteContext(context.Background()); code != 1 || !strings.Contains(stderr.String(), "02.hurl NÃO EXECUTADO") || !strings.Contains(out.String(), "Não executados: 1") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), stderr.String())
	}
}

func TestVerboseSelectionReuseAndColors(t *testing.T) {
	file := suiteFile(t, t.TempDir(), "one.hurl", "GET http://example.test")
	for _, tty := range []bool{false, true} {
		t.Run(fmt.Sprint(tty), func(t *testing.T) {
			t.Setenv("NO_COLOR", "")
			if err := os.Unsetenv("NO_COLOR"); err != nil {
				t.Fatal(err)
			}
			c := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				return runner.Response{Status: 200}, nil
			}), BuildInfo{})
			c.isTerminal = func(io.Writer) bool { return tty }
			var out, stderr bytes.Buffer
			c.SetOut(&out)
			c.SetErr(&stderr)
			for _, flags := range [][]string{{"-v"}, {}, {"--verbose"}} {
				out.Reset()
				stderr.Reset()
				c.SetArgs(append([]string{"test", file}, flags...))
				if code := c.ExecuteContext(context.Background()); code != 0 {
					t.Fatal(code)
				}
				if len(flags) > 0 && (strings.Contains(stderr.String(), "\r") || strings.Contains(stderr.String(), "\x1b") != tty) {
					t.Fatalf("tty=%t err=%q", tty, stderr.String())
				}
				if len(flags) == 0 && strings.Contains(stderr.String(), "[OK]") {
					t.Fatal("verbosity inherited")
				}
			}
		})
	}
}
