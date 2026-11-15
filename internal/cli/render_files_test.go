package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

type renderedFileExecutor struct{ bodies []string }

func (e *renderedFileExecutor) Execute(_ context.Context, req runner.Request) (runner.Response, error) {
	e.bodies = append(e.bodies, string(req.Body))
	return runner.Response{Status: 201}, nil
}

func TestRenderFilesPerInvocation(t *testing.T) {
	dir := t.TempDir()
	source := `{"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}}}`
	if err := os.WriteFile(filepath.Join(dir, "product.json"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "products.hurl")
	if err := os.WriteFile(file, []byte("POST http://example.test/products\nContent-Type: application/json\nfile,product.json;\nHTTP 201"), 0600); err != nil {
		t.Fatal(err)
	}
	executor := &renderedFileExecutor{}
	cmd := NewRootCommand(executor, BuildInfo{})
	for i, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"test", "--render-files", file, "--var", "product.name=Pizza", "--var", "product.price=45", "--var", "product.active=true"}, `{"name":"Pizza","price":45,"active":true}`},
		{[]string{"test", file, "--var", "product.name=Suco", "--var", "product.price=12", "--var", "product.active=false", "--render-files"}, `{"name":"Suco","price":12,"active":false}`},
		{[]string{"test", file}, source},
	} {
		var out, stderr bytes.Buffer
		cmd.SetArgs(tc.args)
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		code := cmd.ExecuteContext(context.Background())
		if code != 0 || stderr.Len() != 0 || !strings.Contains(out.String(), "validadas=1") || len(executor.bodies) != i+1 || executor.bodies[i] != tc.want {
			t.Fatalf("code=%d out=%q err=%q bodies=%q", code, out.String(), stderr.String(), executor.bodies)
		}
	}
}

func TestRenderFilesPayloadDiagnostic(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		dir := t.TempDir()
		payload := strings.Join([]string{"{", ` "escaped":"{{value}}",`, ` "next":{{missing}}`, "}"}, newline)
		if err := os.WriteFile(filepath.Join(dir, "payload.json"), []byte(payload), 0600); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, "source.hurl")
		if err := os.WriteFile(file, []byte("GET http://example.test/first\nHTTP 201\nPOST http://example.test/second\nfile,payload.json;\nGET http://example.test/last"), 0600); err != nil {
			t.Fatal(err)
		}
		executor := &renderedFileExecutor{}
		cmd := NewRootCommand(executor, BuildInfo{})
		var out, stderr bytes.Buffer
		cmd.SetArgs([]string{"test", file, "--render-files", "--var", `value="quotes\"\\\n雪"`, "--secret", "value"})
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		code := cmd.ExecuteContext(context.Background())
		if code != 2 || len(executor.bodies) != 1 || !strings.Contains(stderr.String(), `"payload.json":3:9:`) || !strings.Contains(stderr.String(), fmt.Sprintf("referência %q:4:1", file)) || !strings.Contains(stderr.String(), "variável ausente") || !strings.Contains(out.String(), "iniciadas=1 concluídas=1 validadas=1 sem_validação=0 não_iniciadas=2") || strings.Contains(out.String()+stderr.String(), "quotes") {
			t.Fatalf("code=%d bodies=%q out=%q err=%q", code, executor.bodies, out.String(), stderr.String())
		}
	}
}

func TestRenderFilesRejectsValue(t *testing.T) {
	for _, arg := range []string{"--render-files=true", "--render-files=false", "--render-files="} {
		if _, err := parseArguments([]string{"test", arg, "source.hurl"}); err == nil {
			t.Fatalf("accepted %q", arg)
		}
	}
}
