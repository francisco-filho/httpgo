package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

// GO-421–424: dynamic errors are reached lazily; late static errors send nothing.
func TestJSONInequalityProfilesAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name, expected, body string
		dynamic              bool
		code, calls          int
	}{
		{"literal equal", `"{{v}}"`, `"{{v}}"`, false, 1, 1},
		{"literal different", `"{{v}}"`, `"resolved"`, false, 0, 2},
		{"dynamic equal", `"{{v}}"`, `"resolved"`, true, 1, 1},
		{"dynamic missing", `{{missing}}`, `bad`, true, 2, 1},
		{"literal bare", `{{v}}`, `null`, false, 2, 0},
		{"malformed template", `"{{bad-name}}"`, `null`, true, 2, 0},
		{"extra literal", `1 2`, `null`, false, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := writeScenario(t, "GET http://example.test/first\nHTTP 200\n[Asserts]\njsonpath \"$\" != "+tc.expected+"\nGET http://example.test/next")
			calls := 0
			cmd := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(tc.body)}, nil
			}), BuildInfo{})
			args := []string{"test", file, "--var", "v=resolved"}
			if tc.dynamic {
				args = append(args, "--render-asserts")
			}
			var out, stderr bytes.Buffer
			cmd.SetArgs(args)
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			if code := cmd.ExecuteContext(context.Background()); code != tc.code || calls != tc.calls || tc.code == 1 && !strings.Contains(stderr.String(), "!=") {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out.String(), stderr.String())
			}
		})
	}
	for _, line := range []string{`jsonpath "$" != 1 2`, `jsonpath "$" != {`, `jsonpath "$[0:1]" != []`, `jsonpath "$" count != 0`, `header "X" != "a"`, `body != "x"`, `jsonpath "$" not == null`} {
		file := writeScenario(t, "GET http://example.test/first\nGET http://example.test/late\nHTTP 200\n[Asserts]\n"+line)
		calls := 0
		cmd := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			calls++
			return runner.Response{Status: 200}, nil
		}), BuildInfo{})
		var out, stderr bytes.Buffer
		cmd.SetArgs([]string{"test", file})
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		if code := cmd.ExecuteContext(context.Background()); code != 2 || calls != 0 || !strings.Contains(stderr.String(), ":5:") {
			t.Fatalf("line=%s code=%d calls=%d err=%s", line, code, calls, stderr.String())
		}
	}
}
