package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestVariableFlagsReachExecutor(t *testing.T) {
	file := writeScenario(t, "POST {{ api.baseUrl }}/{{ path }}\nAuthorization: Bearer {{token}}\nX-Probe: {{empty}}-{{eq}}-{{number}}-{{bool}}-{{raw}}\n{\"static\":true}\nHTTP 201")
	for _, base := range []string{"http://one.test", "https://two.test"} {
		calls := 0
		command := NewRootCommand(executorFunc(func(_ context.Context, r runner.Request) (runner.Response, error) {
			calls++
			if r.URL != base+"/a%2Fb" || r.Headers[0].Value != "Bearer fake-token" || r.Headers[1].Value != "-a=b=c-42-true-{{literal}}" || string(r.Body) != `{"static":true}` {
				t.Fatalf("request=%+v", r)
			}
			return runner.Response{Status: 201}, nil
		}), BuildInfo{})
		var out, stderr bytes.Buffer
		command.SetOut(&out)
		command.SetErr(&stderr)
		command.SetArgs([]string{"test", "--secret=token", file, "--var", "api.baseUrl=" + base, "--var=token=fake-token", "--var", "path=a%2Fb", "--var", "empty=", "--var", "eq=a=b=c", "--var", "number=42", "--var", "bool=true", "--var", "raw={{literal}}"})
		if code := command.ExecuteContext(context.Background()); code != 0 || calls != 1 || stderr.Len() != 0 || strings.Contains(out.String(), "fake-token") {
			t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out.String(), stderr.String())
		}
	}
}

func TestInvalidFlagsAndSyntaxPreventAllSends(t *testing.T) {
	for _, flags := range [][]string{{"--var", "noEquals"}, {"--var", "bad-name=fake-token"}, {"--var", "a=fake-token", "--var", "a=fake-token"}, {"--var", "a.b=x", "--var", "a=y"}, {"--var", "a=y", "--var", "a.b=x"}, {"--var"}, {"--secret"}, {"--secret", "bad-name"}, {"--unknown=fake-token"}} {
		fake := &fakeExecutor{}
		args := append([]string{"test", writeScenario(t, "GET http://example.test/")}, flags...)
		code, out, stderr := execute(t, context.Background(), fake, args...)
		if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 || strings.Contains(stderr, "fake-token") {
			t.Fatalf("flags=%q code=%d out=%q err=%q", flags, code, out, stderr)
		}
	}
	for _, suffix := range []string{"GET {{bad-name}}", "GET {{ missing", "GET {{base}}\nX-{{name}}: foo", "POST {{base}}\n{\"{{token}}\":\"static\"}"} {
		fake := &fakeExecutor{}
		file := writeScenario(t, "GET http://example.test/first\n"+suffix)
		code, out, stderr := execute(t, context.Background(), fake, "test", file, "--var", "base=http://example.test", "--var", "token=fake-token", "--secret", "token")
		if code != 2 || out != "" || len(fake.calls) != 0 || !strings.Contains(stderr, file+"\":2:") && !strings.Contains(stderr, file+"\":3:") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}

func TestRenderErrorsAndSecretSafety(t *testing.T) {
	const secret = "fake/a b%?=é"
	for _, mode := range []string{"success", "mismatch", "transport", "missing", "bad URL", "bad header", "empty", "unused missing"} {
		t.Run(mode, func(t *testing.T) {
			value := "fake%2Fa%20b%25%3F%3D%C3%A9"
			text := "GET http://example.test/{{token}}\nX-Ordinary: {{token}}\nHTTP 200\nGET http://example.test/last"
			flags := []string{"--var", "token=" + value, "--secret", "token"}
			fake := &fakeExecutor{statuses: []int{200, 200}}
			wantCode, wantCalls := 0, 2
			switch mode {
			case "mismatch":
				fake.statuses[0] = 503
				wantCode, wantCalls = 1, 1
			case "transport":
				fake.err = errors.New(secret + value)
				wantCode, wantCalls = 2, 1
			case "missing":
				text = "GET http://example.test/first\nHTTP 200\nGET http://example.test/{{absent}}\nGET http://example.test/last"
				wantCode, wantCalls = 2, 1
			case "bad URL":
				flags = []string{"--var", "token=" + secret, "--secret", "token"}
				wantCode, wantCalls = 2, 0
			case "bad header":
				text = "GET http://example.test/\nX-Probe: {{token}}\nGET http://example.test/last"
				flags = []string{"--secret", "token", "--var", "token=fake\r\nInjected: yes"}
				wantCode, wantCalls = 2, 0
			case "empty":
				flags = []string{"--secret", "token", "--var", "token="}
			case "unused missing":
				text = "GET http://example.test/\nGET http://example.test/last"
				flags = []string{"--secret", "token"}
			}
			code, out, stderr := execute(t, context.Background(), fake, append([]string{"test", writeScenario(t, text)}, flags...)...)
			if code != wantCode || len(fake.calls) != wantCalls {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, len(fake.calls), out, stderr)
			}
			for _, protected := range []string{secret, value, url.PathEscape(secret), "Injected", "absent"} {
				if strings.Contains(out+stderr, protected) {
					t.Fatalf("leaked %q in %q %q", protected, out, stderr)
				}
			}
			if mode == "missing" && (!strings.Contains(out, "iniciadas=1 concluídas=1") || !strings.Contains(out, "não_iniciadas=2") || !strings.Contains(stderr, ":3:25: erro de renderização")) {
				t.Fatalf("out=%q err=%q", out, stderr)
			}
			if mode == "bad header" || mode == "bad URL" {
				if !strings.Contains(out, "iniciadas=0") || strings.Contains(out, "erro de transporte") {
					t.Fatalf("out=%q", out)
				}
			}
		})
	}
}

func TestContextIsolationAndProtectionBeforeErrors(t *testing.T) {
	var out, stderr bytes.Buffer
	calls := 0
	command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		return runner.Response{Status: 200}, nil
	}), BuildInfo{})
	command.SetOut(&out)
	command.SetErr(&stderr)
	file := writeScenario(t, "GET http://example.test/{{token}}")
	for i, args := range [][]string{
		{"test", file, "--var", "token=fake-one", "--secret", "token"},
		{"test", file, "--var", "token=visible-two"},
		{"test", file, "--secret", "token"},
	} {
		out.Reset()
		stderr.Reset()
		command.SetArgs(args)
		code := command.ExecuteContext(context.Background())
		if i < 2 && code != 0 || i == 2 && code != 2 || strings.Contains(out.String()+stderr.String(), "fake-one") {
			t.Fatalf("iteration=%d code=%d out=%q err=%q", i, code, out.String(), stderr.String())
		}
		if i == 1 && !strings.Contains(out.String(), "visible-two") {
			t.Fatal("secret declaration leaked across executions")
		}
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
	for _, args := range [][]string{
		{"test", "fake-token", "--unknown", "--var", "token=fake-token", "--secret", "token"},
		{"test", "fake-token", "--var", "token=fake-token", "--var", "token=fake-token", "--secret", "token"},
		{"test", "fake-token", "--secret", "token", "--var", "token=fake-token"},
	} {
		code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, args...)
		if code != 2 || strings.Contains(out+stderr, "fake-token") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}

func TestSecretDeclarationDoesNotDefineValue(t *testing.T) {
	fake := &fakeExecutor{}
	code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/{{token}}"), "--secret", "token")
	if code != 2 || len(fake.calls) != 0 || !strings.Contains(stderr, "variável ausente") || !strings.Contains(out, "iniciadas=0") {
		t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
	}
	code, out, stderr = execute(t, context.Background(), fake, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help code=%d err=%q", code, stderr)
	}
	for _, want := range []string{"--var nome=valor", "--secret nome", "Proibido", "stdin", "fictício"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help lacks %q", want)
		}
	}
}

type shortWriter struct{}

func (shortWriter) Write(data []byte) (int, error) { return len(data) / 2, nil }

func TestProtectedWriterDetectsShortWrite(t *testing.T) {
	writer := protectedWriter{writer: shortWriter{}, protect: func(string) string { return "[oculto]" }}
	if _, err := writer.Write([]byte("fake")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("write error=%v", err)
	}
}

func TestProtectedWriteFailureStopsSequence(t *testing.T) {
	for _, status := range []int{200, 503} {
		fake := &fakeExecutor{statuses: []int{status}}
		out := &failAfterWriter{successes: 0, cause: errors.New("fake-token")}
		var stderr bytes.Buffer
		command := NewRootCommand(fake, BuildInfo{})
		command.SetOut(out)
		command.SetErr(&stderr)
		command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/{{token}}\nHTTP 200\nGET http://example.test/last"), "--var", "token=fake-token", "--secret", "token"})
		if code := command.ExecuteContext(context.Background()); code != 2 || len(fake.calls) != 1 || strings.Contains(stderr.String(), "fake-token") {
			t.Fatalf("code=%d calls=%d err=%q", code, len(fake.calls), stderr.String())
		}
	}
}
