package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

type fakeExecutor struct {
	calls    []string
	statuses []int
	err      error
}

func (f *fakeExecutor) Execute(_ context.Context, req runner.Request) (runner.Response, error) {
	f.calls = append(f.calls, req.URL)
	if f.err != nil {
		return runner.Response{}, f.err
	}
	return runner.Response{Status: f.statuses[len(f.calls)-1]}, nil
}

func execute(t *testing.T, ctx context.Context, fake runner.HTTPExecutor, args ...string) (int, string, string) {
	t.Helper()
	var out, err bytes.Buffer
	command := NewRootCommand(fake, BuildInfo{"1.0-test", "abc123", "2026-11-14"})
	command.SetIn(strings.NewReader(""))
	command.SetOut(&out)
	command.SetErr(&err)
	command.SetArgs(args)
	return command.ExecuteContext(ctx), out.String(), err.String()
}

func writeScenario(t *testing.T, text string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "input.hurl")
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestHelpVersionAndInvalidUsage(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"help", []string{"--help"}, 0, "httpgo test health.hurl"},
		{"test help", []string{"test", "--help"}, 0, "HTTP 404"},
		{"version", []string{"--version"}, 0, "httpgo 1.0-test commit=abc123 date=2026-11-14"},
		{"no args", nil, 2, "uso inválido"},
		{"no file", []string{"test"}, 2, "uso inválido"},
		{"two files", []string{"test", "a", "b"}, 2, "uso inválido"},
		{"unknown command", []string{"check", "a"}, 2, "uso inválido"},
		{"root flag", []string{"--token=flag-secret"}, 2, "flag não suportada"},
		{"test flag", []string{"test", "--token=flag-secret"}, 2, "flag não suportada"},
		{"extra flag", []string{"test", "a", "--unknown"}, 2, "uso inválido"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeExecutor{}
			code, out, err := execute(t, context.Background(), fake, tc.args...)
			if code != tc.code || !strings.Contains(out+err, tc.want) || len(fake.calls) != 0 || strings.Contains(out+err, "flag-secret") {
				t.Fatalf("code=%d,out=%q,err=%q,calls=%v", code, out, err, fake.calls)
			}
			if code == 0 && err != "" || code != 0 && out != "" {
				t.Fatalf("wrong stream: out=%q,err=%q", out, err)
			}
		})
	}
}

func TestResultsAndSecretRedaction(t *testing.T) {
	for _, tc := range []struct {
		name, text       string
		statuses         []int
		failure          error
		code, calls      int
		wantOut, wantErr []string
	}{
		{"success", "GET http://url-user:url-password@example.test/health?token=query-secret#fragment-secret\nHTTP 200\nGET http://example.test/missing\nHTTP 404", []int{200, 404}, nil, 0, 2, []string{"/health?[oculta]", "status validado", "validadas=2", "iniciadas=2", "concluídas=2", "não_iniciadas=0"}, nil},
		{"divergence", "GET http://example.test/health\nHTTP 200\nGET http://example.test/next", []int{503}, nil, 1, 1, []string{"status divergente", "esperado 200, recebido 503", "não_iniciadas=1"}, []string{":2:1:", "esperado 200, recebido 503", "interrompida"}},
		{"no expectation", "GET http://example.test/health", []int{500}, nil, 0, 1, []string{"sem validação de status", "recebido 500", "validadas=0", "sem_validação=1"}, nil},
		{"transport", "GET http://url-user:url-password@example.test/health?token=query-secret#fragment-secret\nHTTP 200\nGET http://example.test/next", nil, errors.New("raw-secret"), 2, 1, []string{"erro de transporte", "concluídas=0", "não_iniciadas=1"}, []string{":1:1:", "falha de transporte", "interrompida"}},
		{"cancelled", "GET http://example.test/health\nGET http://example.test/next", nil, context.Canceled, 2, 1, []string{"erro de transporte", "não_iniciadas=1"}, []string{"cancelada"}},
		{"timeout", "GET http://example.test/health\nGET http://example.test/next", nil, context.DeadlineExceeded, 2, 1, []string{"erro de transporte"}, []string{"timeout", "10 segundos"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := writeScenario(t, tc.text)
			fake := &fakeExecutor{statuses: tc.statuses, err: tc.failure}
			code, out, err := execute(t, context.Background(), fake, "test", file)
			if code != tc.code || len(fake.calls) != tc.calls {
				t.Fatalf("code=%d,out=%q,err=%q,calls=%v", code, out, err, fake.calls)
			}
			for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
				if strings.HasPrefix(line, "resumo ") {
					continue
				}
				prefix := "GET "
				if tc.code != 0 {
					prefix = fmt.Sprintf("%q:1:1: GET ", file)
				}
				if !strings.HasPrefix(line, prefix) {
					t.Errorf("result line = %q; want prefix %q", line, prefix)
				}
			}
			for _, want := range tc.wantOut {
				if !strings.Contains(out, want) {
					t.Errorf("stdout=%q,want %q", out, want)
				}
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err, want) {
					t.Errorf("stderr=%q,want %q", err, want)
				}
			}
			if len(tc.wantErr) == 0 && err != "" {
				t.Errorf("unexpected stderr=%q", err)
			}
			for _, secret := range []string{"url-user", "url-password", "query-secret", "fragment-secret", "raw-secret"} {
				if strings.Contains(out+err, secret) {
					t.Errorf("leaked %q in out=%q,err=%q", secret, out, err)
				}
			}
		})
	}
}

func TestInvalidInputNeverCallsExecutor(t *testing.T) {
	for _, text := range []string{"", "# only comments", "GET http://example.test/first\nHEAD http://example.test/second", "GET http://example.test/first\nGET http://url-user:url-password@bad%zz/?token=query-secret", "GET http://example.test/first\nHTTP 999", "GET http://example.test/first\nHTTP 200 # forbidden"} {
		t.Run(text, func(t *testing.T) {
			fake := &fakeExecutor{}
			file := writeScenario(t, text)
			code, out, err := execute(t, context.Background(), fake, "test", file)
			if code != 2 || out != "" || !strings.Contains(err, file) || len(fake.calls) != 0 {
				t.Fatalf("code=%d,out=%q,err=%q,calls=%v", code, out, err, fake.calls)
			}
			for _, secret := range []string{"url-user", "url-password", "bad%zz", "query-secret"} {
				if strings.Contains(err, secret) {
					t.Errorf("leaked %q: %s", secret, err)
				}
			}
		})
	}
	for _, path := range []string{filepath.Join(t.TempDir(), "missing.hurl"), t.TempDir()} {
		fake := &fakeExecutor{}
		code, out, err := execute(t, context.Background(), fake, "test", path)
		if code != 2 || out != "" || err == "" || len(fake.calls) != 0 {
			t.Fatalf("code=%d,out=%q,err=%q", code, out, err)
		}
	}
}

func TestAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &fakeExecutor{}
	code, out, err := execute(t, ctx, fake, "test", writeScenario(t, "GET http://example.test/health"))
	if code != 2 || len(fake.calls) != 0 || !strings.Contains(err, "cancelada") || out != "" {
		t.Fatalf("code=%d,out=%q,err=%q", code, out, err)
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("raw-secret") }

func TestOutputFailure(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}, {"test", writeScenario(t, "GET http://example.test/health")}} {
		var err bytes.Buffer
		command := NewRootCommand(&fakeExecutor{statuses: []int{200}}, BuildInfo{})
		command.SetArgs(args)
		command.SetOut(brokenWriter{})
		command.SetErr(&err)
		if code := command.ExecuteContext(context.Background()); code != 2 || !strings.Contains(err.String(), "escrever") || strings.Contains(err.String(), "raw-secret") {
			t.Fatalf("code=%d,err=%q", code, err.String())
		}
	}
	command := NewRootCommand(&fakeExecutor{statuses: []int{503}}, BuildInfo{})
	command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/\nHTTP 200")})
	command.SetOut(io.Discard)
	command.SetErr(brokenWriter{})
	if code := command.ExecuteContext(context.Background()); code != 2 {
		t.Fatalf("stderr write failure code=%d", code)
	}
}

func TestNoColorAndPlainOutput(t *testing.T) {
	// Both stream-backed and file-backed outputs use exactly the same plain format.
	for _, noColor := range []string{"", "1"} {
		t.Run("NO_COLOR="+noColor, func(t *testing.T) {
			t.Setenv("NO_COLOR", noColor)
			code, out, err := execute(t, context.Background(), &fakeExecutor{}, "--help")
			if code != 0 || err != "" || strings.Contains(out, "\x1b") || !strings.Contains(out, "não coloque segredos") {
				t.Fatalf("code=%d,out=%q,err=%q", code, out, err)
			}
			file, errOpen := os.CreateTemp(t.TempDir(), "stdout")
			if errOpen != nil {
				t.Fatal(errOpen)
			}
			defer func() { _ = file.Close() }()
			command := NewRootCommand(&fakeExecutor{}, BuildInfo{})
			command.SetArgs([]string{"--help"})
			command.SetOut(file)
			if command.ExecuteContext(context.Background()) != 0 {
				t.Fatal("file output failed")
			}
			data, readErr := os.ReadFile(file.Name())
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(data) != out {
				t.Fatalf("file output differs: %q", data)
			}
		})
	}
}
