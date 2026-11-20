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

	"capimgrosso.com/httpgo/internal/config"
	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestEnvFileEarlyProtection(t *testing.T) {
	const token = "fiction-file-sensitive"
	for _, mode := range []string{"arguments", "composition", "scenario", "missing first", "missing last", "merge", "override"} {
		t.Run(mode, func(t *testing.T) {
			base := writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_43_KNOWN}', shape: 1}")
			file := suiteFile(t, t.TempDir(), token+".httpgo", "GET http://example.test/{{token}}\nHTTP 200")
			args := []string{"test", file, "--config", base, "--env-file", writeEnvFile(t, "HTTPGO_43_KNOWN="+token)}
			switch mode {
			case "arguments":
				args = append(args, "--unknown")
			case "composition":
				args = append(args, "--var", "shape.child=1")
			case "scenario":
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			case "missing first", "missing last":
				key := "aMissing"
				if mode == "missing last" {
					key = "zMissing"
				}
				// Known value must be prepared even if another lookup fails first.
				base = suiteFile(t, t.TempDir(), token+".yaml", fmt.Sprintf("version: 1\nvariables: {token: '${HTTPGO_43_KNOWN}', %s: '${HTTPGO_43_ABSENT}'}", key))
				args[3] = base
			case "merge":
				writeEnvironment(t, base, token, "variables: {shape: {child: true}}")
				args = append(args, "--env", token)
			case "override":
				args = append(args, "--var", "token="+token, "--var", "token=second-fiction")
			}
			fake := &fakeExecutor{}
			code, out, stderr := execute(t, context.Background(), fake, args...)
			if code != 2 || len(fake.calls) != 0 || stderr == "" || strings.Contains(out+stderr, token) || strings.Contains(out+stderr, "second-fiction") {
				t.Fatalf("mode=%s code=%d calls=%d safe=%t", mode, code, len(fake.calls), !strings.Contains(out+stderr, token))
			}
		})
	}
}

func TestEnvFilePresentationProtection(t *testing.T) {
	const token = "fiction-echo-file-token"
	for _, mode := range []string{"text", "verbose", "junit", "html"} {
		for _, override := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/override=%t", mode, override), func(t *testing.T) {
				value := token
				base := writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_43_KNOWN}'}")
				file := writeScenario(t, "POST http://example.test/{{token}}\n{\"token\":\"prefix-{{token}}\"}\nHTTP 200\n[Asserts]\njsonpath \"$.echo\" == \"different\"\nGET http://example.test/never")
				args := []string{"test", file, "--config", base, "--env-file", writeEnvFile(t, "HTTPGO_43_KNOWN="+token)}
				if override {
					value = "substitute-file-fiction"
					args = append(args, "--var", "token="+value)
				}
				destination := filepath.Join(t.TempDir(), "report."+mode)
				switch mode {
				case "verbose":
					args = append(args, "-vv")
				case "junit", "html":
					args = append(args, "--output", mode, "--output-file", destination)
				}
				calls := 0
				fake := executorFunc(func(_ context.Context, r runner.Request) (runner.Response, error) {
					calls++
					if r.URL != "http://example.test/"+value || string(r.Body) != `{"token":"prefix-`+value+`"}` {
						t.Error("operational value changed")
					}
					return verboseResponse(r, 200, `{"echo":"prefix-`+value+`"}`), nil
				})
				code, out, stderr := execute(t, context.Background(), fake, args...)
				text := out + stderr
				switch mode {
				case "junit":
					data, err := os.ReadFile(destination)
					if err != nil {
						t.Fatal(err)
					}
					text += string(data)
				case "html":
					_, html := readHTML(t, destination)
					text += html
				}
				if code != 1 || calls != 1 || strings.Contains(text, value) || !strings.Contains(text, "[oculto]") {
					t.Fatalf("code=%d calls=%d safe=%t", code, calls, !strings.Contains(text, value))
				}
			})
		}
	}
}

func TestEnvFileReloadAndSuiteIsolation(t *testing.T) {
	base := writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_43_KNOWN}'}\nsecrets: [captured]")
	path := writeEnvFile(t, "HTTPGO_43_KNOWN=first-fiction")
	dir := t.TempDir()
	for _, name := range []string{"01.httpgo", "02.httpgo"} {
		suiteFile(t, dir, name, "GET http://example.test/{{token}}\nHTTP 200\n[Captures]\ncaptured: jsonpath \"$.value\"\nGET http://example.test/{{captured}}\nHTTP 200")
	}
	want, calls := "first-fiction", 0
	command := NewRootCommand(executorFunc(func(_ context.Context, r runner.Request) (runner.Response, error) {
		calls++
		if calls%2 == 1 && r.URL != "http://example.test/"+want {
			t.Error("source reloaded during suite or stale on new invocation")
		}
		// Once loading is complete, later changes must not affect this suite.
		if err := os.WriteFile(path, []byte("HTTPGO_43_KNOWN=second-fiction"), 0600); err != nil {
			t.Fatal(err)
		}
		return runner.Response{Status: 200, Body: []byte(`{"value":"captured-fiction"}`)}, nil
	}), BuildInfo{})
	var out, stderr bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&stderr)
	command.SetArgs([]string{"test", dir, "--config", base, "--env-file", path})
	for _, next := range []string{"first-fiction", "second-fiction"} {
		want = next
		out.Reset()
		stderr.Reset()
		if code := command.ExecuteContext(context.Background()); code != 0 || strings.Contains(out.String()+stderr.String(), want) || strings.Contains(out.String(), "captured-fiction") {
			t.Fatalf("code=%d safe/isolation failed", code)
		}
	}
	if calls != 8 {
		t.Fatalf("calls=%d", calls)
	}
	// Select a different file, then remove the flag and configuration entirely.
	want = "other-fiction"
	command.SetArgs([]string{"test", dir, "--config", base, "--env-file", writeEnvFile(t, "HTTPGO_43_KNOWN=other-fiction")})
	if code := command.ExecuteContext(context.Background()); code != 0 || calls != 12 {
		t.Fatalf("different file code=%d calls=%d", code, calls)
	}
	command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/{{captured}}")})
	if code := command.ExecuteContext(context.Background()); code != 2 || calls != 12 {
		t.Fatal("capture leaked across invocations")
	}
	out.Reset()
	want = "second-fiction"
	command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/second-fiction\nHTTP 200")})
	if code := command.ExecuteContext(context.Background()); code != 0 || !strings.Contains(out.String(), want) {
		t.Fatal("protection leaked into unconfigured invocation")
	}
	if _, present := os.LookupEnv("HTTPGO_43_KNOWN"); present || command.protection != nil {
		t.Fatal("retained invocation state or mutated process environment")
	}
}

func TestEnvFileWriterFailures(t *testing.T) {
	for _, mode := range []string{"result error", "result short", "diagnostic error", "diagnostic short", "preparation error", "preparation short"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 503}, nil
			}), BuildInfo{})
			writer := &suiteWriter{short: strings.HasSuffix(mode, "short")}
			var out, stderr bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&stderr)
			if strings.HasPrefix(mode, "result") {
				command.SetOut(writer)
			} else {
				command.SetErr(writer)
			}
			source := "version: 1\nvariables: {token: '${HTTPGO_43_KNOWN}'}"
			if strings.HasPrefix(mode, "preparation") {
				source += "\nunsupported: true"
			}
			command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/{{token}}\nHTTP 200\nGET http://example.test/never"), "--config", writeConfig(t, source), "--env-file", writeEnvFile(t, "HTTPGO_43_KNOWN=fiction-failure-token")})
			wantCalls := 1
			if strings.HasPrefix(mode, "preparation") {
				wantCalls = 0
			}
			if code := command.ExecuteContext(context.Background()); code != 2 || calls != wantCalls || writer.calls != 1 || strings.Contains(out.String()+stderr.String(), "fiction-failure-token") {
				t.Fatalf("mode=%s code=%d calls=%d writes=%d", mode, code, calls, writer.calls)
			}
		})
	}
}

func TestEnvFileResultsProtectedBeforePresentation(t *testing.T) {
	const token = "fiction-callback-token"
	ctx := context.Background()
	settings, err := config.Load(ctx, writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_43_KNOWN}'}"))
	if err != nil {
		t.Fatal(err)
	}
	settings, err = settings.WithEnvFile(ctx, writeEnvFile(t, "HTTPGO_43_KNOWN="+token))
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := settings.Compose(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := parser.Parse("callback.httpgo", strings.NewReader("GET http://example.test/{{token}}\nHTTP 200\n[Asserts]\njsonpath \"$.echo\" == \"different\""))
	if err != nil {
		t.Fatal(err)
	}
	callbacks := 0
	execution := runner.NewWithRenderer(executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
		if !strings.HasSuffix(request.URL, token) {
			t.Fatal("operational value changed")
		}
		return runner.Response{Status: 200, Body: []byte(`{"echo":"prefix-` + token + `"}`)}, nil
	}), renderer, func(entry runner.EntryResult) error {
		callbacks++
		if strings.Contains(fmt.Sprintf("%+v %+v", entry, entry.JSONFailure), token) {
			t.Fatal("callback exposed unsanitized operand")
		}
		return nil
	})
	result := execution.Run(ctx, scenario)
	if !result.Diverged || callbacks != 1 || strings.Contains(fmt.Sprintf("%+v %+v", result, result.Entries[0].JSONFailure), token) {
		t.Fatal("result exposed unsanitized operand")
	}
}

func TestEnvFileTransportCauseIsPrivate(t *testing.T) {
	const token = "fiction-private-transport"
	fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		return runner.Response{}, errors.New(token)
	})
	code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/{{token}}"), "--config", writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_43_KNOWN}'}"), "--env-file", writeEnvFile(t, "HTTPGO_43_KNOWN="+token))
	if code != 2 || strings.Contains(out+stderr, token) {
		t.Fatal("transport failure leaked")
	}
}
