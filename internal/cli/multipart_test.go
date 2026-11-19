package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

type multipartEcho struct{ calls int }

func (e *multipartEcho) Execute(_ context.Context, req runner.Request) (runner.Response, error) {
	e.calls++
	if req.Multipart == nil {
		return runner.Response{Status: 200}, nil
	}
	data, _ := json.Marshal(map[string]string{"echo": req.Multipart.Parts[0].Value})
	return runner.Response{Status: 201, Body: data}, nil
}

func TestMultipartCLISafeDiagnostics(t *testing.T) {
	for _, mode := range []string{"success", "assertion", "missing", "variable", "type", "structural"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "upload.hurl")
			payload := filepath.Join(dir, "private.pdf")
			if err := os.WriteFile(payload, []byte("never-print-file"), 0600); err != nil {
				t.Fatal(err)
			}
			text := "GET http://localhost/first\nHTTP 200\nPOST http://localhost/upload\nAuthorization: Bearer {{secret}}\nContent-Type: {{kind}}\n[MultipartFormData]\ndescription: prefix-{{secret}}-suffix\nextra: {{value}}\ndocument: file,private.pdf;\nHTTP 201"
			vars := []string{"--var", "secret=fictitious-secret", "--secret", "secret", "--var", "kind=multipart/form-data"}
			if mode != "variable" {
				vars = append(vars, "--var", "value=normal")
			}
			switch mode {
			case "missing":
				if err := os.Remove(payload); err != nil {
					t.Fatal(err)
				}
			case "type":
				vars[len(vars)-3] = "kind=application/json"
			case "structural":
				text = strings.ReplaceAll(text, "extra: {{value}}", "description: duplicated")
			case "assertion":
				text += "\n[Asserts]\njsonpath \"$.echo\" == \"different\""
			case "success":
				text += "\n[Asserts]\njsonpath \"$.echo\" == \"prefix-fictitious-secret-suffix\""
			}
			if err := os.WriteFile(file, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			executor := &multipartEcho{}
			cmd := NewRootCommand(executor, BuildInfo{})
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetArgs(append([]string{"test", file}, vars...))
			code := cmd.ExecuteContext(context.Background())
			want := 2
			calls := 1
			if mode == "success" {
				want = 0
				calls = 2
			}
			if mode == "assertion" {
				want = 1
				calls = 2
			}
			if mode == "structural" {
				calls = 0
			}
			if code != want || executor.calls != calls {
				t.Fatalf("code=%d want=%d calls=%d out=%q err=%q", code, want, executor.calls, out.String(), stderr.String())
			}
			for _, secret := range []string{"fictitious-secret", "never-print-file", "prefix-"} {
				if strings.Contains(out.String()+stderr.String(), secret) {
					t.Fatalf("unsafe output=%q", out.String()+stderr.String())
				}
			}
			if code == 2 && mode != "structural" && !strings.Contains(out.String(), "iniciadas=1 concluídas=1") {
				t.Fatalf("out=%q", out.String())
			}
			if code != 0 && !strings.Contains(stderr.String(), fmt.Sprintf("%q:", shownPath(t, file))) {
				t.Fatalf("unlocated error=%q", stderr.String())
			}
		})
	}
}
