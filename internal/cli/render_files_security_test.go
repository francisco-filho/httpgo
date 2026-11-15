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

	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

func TestRenderFilesSecretsAndWriterPriority(t *testing.T) {
	const secret = `"credential-fictitious\"\\\n雪"`
	for _, captured := range []bool{false, true} {
		for _, mode := range []string{"success", "mismatch", "render", "transport", "writer", "render writer", "mismatch writer"} {
			t.Run(fmt.Sprintf("captured=%t/%s", captured, mode), func(t *testing.T) {
				dir := t.TempDir()
				payload := `{"token":"prefix-{{credential}}"}`
				if strings.HasPrefix(mode, "render") {
					payload = `{"token":"prefix-{{credential}}","next":{{missing}}}`
				}
				if err := os.WriteFile(filepath.Join(dir, "payload.json"), []byte(payload), 0600); err != nil {
					t.Fatal(err)
				}
				prefix := "GET http://example.test/first\nHTTP 200\n"
				args := []string{"test", "--render-files", "--secret", "credential"}
				if captured {
					prefix += "[Captures]\ncredential: jsonpath \"$\"\n"
				} else {
					args = append(args, "--var", "credential="+secret)
				}
				file := writeScenario(t, prefix+"POST http://example.test/affected\nfile,"+filepath.Join(dir, "payload.json")+";\nHTTP 201\nGET http://example.test/last")
				calls := 0
				cmd := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					if calls == 1 {
						return runner.Response{Status: 200, Body: []byte(secret)}, nil
					}
					if strings.HasPrefix(mode, "mismatch") {
						return runner.Response{Status: 503}, nil
					}
					if mode == "transport" {
						return runner.Response{}, errors.New("private transport cause")
					}
					return runner.Response{Status: 201}, nil
				}), BuildInfo{})
				cmd.SetArgs(append(args, file))
				out := &failAfterWriter{successes: 10, cause: errors.New("private writer cause")}
				if strings.Contains(mode, "writer") {
					out.successes = 1
				}
				var stderr bytes.Buffer
				cmd.SetOut(out)
				cmd.SetErr(&stderr)
				code := cmd.ExecuteContext(context.Background())
				wantCode, wantCalls := 2, 2
				if mode == "success" {
					wantCode, wantCalls = 0, 3
				}
				if mode == "mismatch" {
					wantCode = 1
				}
				if strings.HasPrefix(mode, "render") {
					wantCalls = 1
				}
				if code != wantCode || calls != wantCalls {
					t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out.String(), stderr.String())
				}
				if strings.Contains(mode, "writer") && (!strings.Contains(stderr.String(), "escrever o resultado") || strings.Contains(out.String(), "resumo")) {
					t.Fatalf("writer lost priority out=%q err=%q", out.String(), stderr.String())
				}
				for _, variant := range append(template.SecretVariants(secret), "private transport cause", "private writer cause", payload) {
					if variant != "" && strings.Contains(out.String()+stderr.String(), variant) {
						t.Fatalf("unsafe output=%q", out.String()+stderr.String())
					}
				}
			})
		}
	}
}
