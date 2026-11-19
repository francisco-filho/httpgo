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

type fileExecutor struct{ calls int }

func (e *fileExecutor) Execute(context.Context, runner.Request) (runner.Response, error) {
	e.calls++
	return runner.Response{Status: 200}, nil
}

func TestExternalFileHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		cmd := NewRootCommand(&fileExecutor{}, BuildInfo{})
		var out, stderr bytes.Buffer
		cmd.SetArgs(args)
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		if code := cmd.ExecuteContext(context.Background()); code != 0 || stderr.Len() != 0 {
			t.Fatalf("code=%d err=%q", code, stderr.String())
		}
		for _, text := range []string{"--render-files", "bodies de arquivos externos como templates JSON", "docs/subset.md"} {
			if !strings.Contains(out.String(), text) {
				t.Fatalf("help missing %q", text)
			}
		}
	}
}

func TestFileDiagnostics(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(fmt.Sprint(directory), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "credential-secret")
			want := "arquivo inexistente"
			if directory {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				want = "diretório"
			}
			file := filepath.Join(dir, "source.hurl")
			input := "GET http://example.test/first\nHTTP 200\nPOST http://example.test/{{credential}}\nAuthorization: {{credential}}\nfile,credential-secret;\nGET http://example.test/last"
			if err := os.WriteFile(file, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			executor := &fileExecutor{}
			cmd := NewRootCommand(executor, BuildInfo{})
			var out, stderr bytes.Buffer
			cmd.SetArgs([]string{"test", file, "--var", "credential=credential-secret"})
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			code := cmd.ExecuteContext(context.Background())
			if code != 2 || executor.calls != 1 || !strings.Contains(stderr.String(), want) || !strings.Contains(stderr.String(), fmt.Sprintf("%q:5:1:", shownPath(t, file))) || !strings.Contains(out.String(), "iniciadas=1 concluídas=1 validadas=1 sem_validação=0 não_iniciadas=2") {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, executor.calls, out.String(), stderr.String())
			}
			if strings.Contains(out.String()+stderr.String(), "credential-secret") || strings.Contains(stderr.String(), "transporte") {
				t.Fatalf("unsafe diagnostic=%q", stderr.String())
			}
		})
	}
}
