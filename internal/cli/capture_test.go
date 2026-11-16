package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

func TestCaptureDiagnosticsAndCodes(t *testing.T) {
	for _, tc := range []struct {
		name, body, suffix  string
		status, code, calls int
		diagnostic, summary string
	}{
		{"login", `{"token":"token-ficticio"}`, "", 200, 0, 2, "", "validadas=2"},
		{"invalid", `secret-body`, "", 200, 2, 1, "resposta não contém JSON válido", "concluídas=1 validadas=1"},
		{"missing", `{}`, "", 200, 2, 1, "consulta não encontrou valor", "concluídas=1 validadas=1"},
		{"null", `{"token":null}`, "", 200, 0, 2, "", "validadas=2"},
		{"number", `{"token":9007199254740993}`, "", 200, 0, 2, "", "validadas=2"},
		{"mismatch wins", `not JSON`, "", 503, 1, 1, "status divergente", "concluídas=1 validadas=0"},
		{"absent later", `{"token":"token-ficticio"}`, "GET http://example.test/{{absent}}", 200, 2, 2, "erro de renderização", "concluídas=2 validadas=2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := "POST http://example.test/login\n{\"password\":\"{{password}}\"}\nHTTP 200\n[Captures]\nauth.token: jsonpath \"$.token\"\nGET http://example.test/products\nAuthorization: Bearer {{auth.token}}\nHTTP 200\n" + tc.suffix
			calls := 0
			fake := executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
				calls++
				if calls == 2 {
					want := "Bearer token-ficticio"
					if tc.name == "null" {
						want = "Bearer null"
					}
					if tc.name == "number" {
						want = "Bearer 9007199254740993"
					}
					if req.Headers[0].Value != want {
						t.Fatalf("%+v", req)
					}
				}
				return runner.Response{Status: tc.status, Body: []byte(tc.body)}, nil
			})
			code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, text), "--secret", "auth.token", "--secret", "password", "--var", "password=senha-ficticia")
			if code != tc.code || calls != tc.calls || !strings.Contains(stderr, tc.diagnostic) || !strings.Contains(out, tc.summary) || strings.Contains(out+stderr, "token-ficticio") || strings.Contains(out+stderr, "senha-ficticia") || strings.Contains(out+stderr, "secret-body") {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out, stderr)
			}
			if code == 2 && tc.calls == 1 && (!strings.Contains(out, "erro de captura (status validado") || strings.Contains(out, "erro de transporte") || !strings.Contains(stderr, ":5:23: erro de captura")) {
				t.Fatalf("out=%q err=%q", out, stderr)
			}
		})
	}
}

func TestDynamicCaptureProtectionAllOutputs(t *testing.T) {
	for _, secret := range []string{"fictitious/a b%?=é", `"quoted"`, "null", "123", "quotes\"\\\n雪", ""} {
		for _, mode := range []string{"success", "status", "capture", "render", "transport", "write", "short"} {
			t.Run(secret+mode, func(t *testing.T) {
				encoded, _ := json.Marshal(secret)
				text := "POST http://example.test/login\n{\"password\":\"{{password}}\"}\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\n"
				// A later failed declaration must not undo protection of discovered strings.
				if mode == "capture" {
					text += "other: jsonpath \"$.missing\"\n"
				}
				// A known URL-escaped variant in static text exercises global protection
				// independently of provenance from a token placeholder.
				text += "POST http://example.test/" + url.PathEscape(secret) + "\nX-Ordinary: fixture\n{\"string\":\"{{token}}\",\"full\":{{token}}}\nHTTP 200\n"
				if mode == "render" {
					text += "GET http://example.test/{{missing}}\n"
				}
				text += "GET http://example.test/last"
				file := filepath.Join(t.TempDir(), "flow-"+url.QueryEscape(secret)+".hurl")
				if err := os.WriteFile(file, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
				calls := 0
				var out, stderr bytes.Buffer
				command := NewRootCommand(executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
					calls++
					if strings.Count(out.String(), "POST ") != calls-1 && mode != "write" && mode != "short" && calls < 3 {
						t.Fatal("next send before prior output")
					}
					if calls == 1 {
						return runner.Response{Status: 200, Body: []byte(`{"token":` + string(encoded) + `}`)}, nil
					}
					if calls == 2 && mode == "transport" {
						return runner.Response{}, errors.New(secret)
					}
					status := 200
					if calls == 2 && mode == "status" {
						status = 503
					}
					return runner.Response{Status: status}, nil
				}), BuildInfo{})
				command.SetOut(&out)
				command.SetErr(&stderr)
				if mode == "write" {
					command.SetOut(&failAfterWriter{cause: errors.New(secret)})
				}
				if mode == "short" {
					command.SetOut(shortWriter{})
				}
				command.SetArgs([]string{"test", file, "--secret", "token", "--var", "password=senha-ficticia", "--secret", "password"})
				code := command.ExecuteContext(context.Background())
				wantCode, wantCalls := 0, 3
				switch mode {
				case "status":
					wantCode, wantCalls = 1, 2
				case "capture", "write", "short":
					wantCode, wantCalls = 2, 1
				case "render", "transport":
					wantCode, wantCalls = 2, 2
				}
				if code != wantCode || calls != wantCalls {
					t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out.String(), stderr.String())
				}
				combined := out.String() + stderr.String()
				for _, variant := range template.SecretVariants(string(encoded)) {
					if secret != "" && strings.Contains(combined, variant) {
						t.Fatalf("leaked %q out=%q err=%q", variant, out.String(), stderr.String())
					}
				}
				if strings.Contains(combined, "senha-ficticia") {
					t.Fatal("password exposed")
				}
				if mode == "success" && !strings.Contains(out.String(), "total=3") {
					t.Fatal("empty secret corrupted output")
				}
			})
		}
	}
}

func TestCaptureCLIIsolationAndEarlyReferences(t *testing.T) {
	var out, stderr bytes.Buffer
	calls := 0
	command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		return runner.Response{Status: 200, Body: []byte(`"private-token"`)}, nil
	}), BuildInfo{})
	command.SetOut(&out)
	command.SetErr(&stderr)
	first := writeScenario(t, "GET http://example.test/login\n[Captures]\ntoken: jsonpath \"$\"\nGET http://example.test/{{token}}")
	command.SetArgs([]string{"test", first, "--secret", "token"})
	if code := command.ExecuteContext(context.Background()); code != 0 || strings.Contains(out.String(), "private-token") {
		t.Fatalf("code=%d %s %s", code, &out, &stderr)
	}
	for _, source := range []string{"GET http://example.test/{{token}}", "GET http://example.test/{{token}}\n[Captures]\ntoken: jsonpath \"$\"\nGET http://example.test/last"} {
		out.Reset()
		stderr.Reset()
		command.SetArgs([]string{"test", writeScenario(t, source), "--secret", "token"})
		if code := command.ExecuteContext(context.Background()); code != 2 || calls != 2 || !strings.Contains(stderr.String(), "variável ausente") || !strings.Contains(out.String(), "iniciadas=0") {
			t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out.String(), stderr.String())
		}
	}
	// Neither secret declarations nor the protection of old captures survive.
	out.Reset()
	stderr.Reset()
	command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/private-token")})
	if code := command.ExecuteContext(context.Background()); code != 0 || !strings.Contains(out.String(), "private-token") {
		t.Fatalf("code=%d %s %s", code, &out, &stderr)
	}
}

func TestCaptureStructuralErrorsAndCollisions(t *testing.T) {
	for _, declaration := range []string{`bad-name: jsonpath "$"`, `token: jsonpath "$.*"`, `token: jsonpath "$[-1]"`, `token: jsonpath "$..x"`} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/first\nGET http://example.test/second\n[Captures]\n"+declaration))
		if code != 2 || len(fake.calls) != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":4:") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
	for _, assignment := range []string{"auth=null", "auth.token=[]", "auth.token.child={}"} {
		calls := 0
		fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			calls++
			return runner.Response{Status: 200, Body: []byte(`"new-token"`)}, nil
		})
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/login\n[Captures]\nauth.token: jsonpath \"$\"\nGET http://example.test/next"), "--var", assignment, "--secret", "auth.token")
		if code != 2 || calls != 1 || !strings.Contains(out, "erro de captura (sem validação de status") || !strings.Contains(stderr, ":3:1: erro de captura") || strings.Contains(out+stderr, "new-token") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}

func TestCaptureWriteFailurePrecedesMismatch(t *testing.T) {
	calls := 0
	command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		if calls == 1 {
			return runner.Response{Status: 200, Body: []byte(`"private-token"`)}, nil
		}
		return runner.Response{Status: 503}, nil
	}), BuildInfo{})
	command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/login\n[Captures]\ntoken: jsonpath \"$\"\nGET http://example.test/{{token}}\nHTTP 200\nGET http://example.test/last"), "--secret", "token"})
	output := &failAfterWriter{successes: 1, cause: errors.New("private-token")}
	var stderr bytes.Buffer
	command.SetOut(output)
	command.SetErr(&stderr)
	if code := command.ExecuteContext(context.Background()); code != 2 || calls != 2 || output.calls != 2 || !strings.Contains(stderr.String(), "escrever o resultado") || strings.Contains(output.String()+stderr.String(), "private-token") {
		t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, output.String(), stderr.String())
	}
}
