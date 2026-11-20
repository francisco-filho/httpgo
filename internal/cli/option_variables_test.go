package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

// GO-571: directory discovery starts each file from composed initial values.
func TestOptionVariablesDirectory(t *testing.T) {
	base := writeConfig(t, "version: 1\nvariables: {tipo_validacao: 5, base_url: 'http://example.test'}")
	writeEnvironment(t, base, "local", "variables: {tipo_validacao: 7}")
	dir := t.TempDir()
	for i, value := range []string{"1", "3", ""} {
		options := ""
		if value != "" {
			options = "[Options]\nvariable: tipo_validacao=" + value + "\nvariable: cliente_externo=" + map[bool]string{false: "false", true: "true"}[i == 1] + "\n"
		}
		body := `{"tipoValidacao":{{tipo_validacao}}}`
		if value != "" {
			body = `{"tipoValidacao":{{tipo_validacao}},"clienteExterno":{{cliente_externo}}}`
		}
		source := "POST {{base_url}}/produtos\n" + options + body + "\nHTTP 201\nPOST {{base_url}}/reuse\n" + body + "\nHTTP 201"
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%02d.httpgo", i)), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	fake := executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		want := []string{`{"tipoValidacao":1,"clienteExterno":false}`, `{"tipoValidacao":3,"clienteExterno":true}`, `{"tipoValidacao":9}`}[calls/2]
		if string(req.Body) != want || len(req.Headers) != 0 {
			t.Fatalf("request=%+v want=%s", req, want)
		}
		calls++
		return runner.Response{Status: 201}, nil
	})
	code, out, stderr := execute(t, context.Background(), fake, "test", dir, "--config", base, "--env", "local", "--var", "tipo_validacao=9")
	if code != 0 || stderr != "" || calls != 6 || !strings.Contains(out, "Testes executados: 6") {
		t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
	}
}

func TestOptionVariableInvalidLateCLI(t *testing.T) {
	for _, option := range []string{`variable: a.b=private`, `variable: private`, `variable: a="private\q"` + "\nvariable: a=valid", "timeout: 1", "variable: a=valid\n[Options]\nvariable: b=valid", "variable: a=valid\nretry: 1\nretry: 2"} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/first\nGET http://example.test/late\n[Options]\n"+option+"\nHTTP 200"))
		if code != 2 || len(fake.calls) != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":4:1:") && !strings.Contains(option, "\n[Options]") && !strings.Contains(option, "retry:") || strings.Contains(stderr, "private") {
			t.Fatalf("option=%s code=%d out=%s err=%s", option, code, out, stderr)
		}
	}
}

func TestOptionVariableDoesNotRepairMissingConfiguration(t *testing.T) {
	const name = "HTTPGO_OPTION_VARIABLE_REQUIRED_TEST"
	t.Setenv(name, "unused")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	base := writeConfig(t, "version: 1\nvariables: {value: '${"+name+"}'}")
	fake := &fakeExecutor{}
	code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test\n[Options]\nvariable: value=replacement"), "--config", base)
	if code != 2 || out != "" || len(fake.calls) != 0 || !strings.Contains(stderr, name) {
		t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
	}
}
