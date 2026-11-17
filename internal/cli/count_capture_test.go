package cli

import (
	"context"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestCountCaptureCodesAndSecretComparison(t *testing.T) {
	for _, tc := range []struct {
		name, body      string
		enabled, secret bool
		code, calls     int
	}{
		{"unchanged", `{"items":[null,null,null]}`, true, false, 0, 3},
		{"changed", `{"items":[null,null,null,null]}`, true, false, 1, 2},
		{"invalid capture", `{"items":null}`, true, false, 2, 1},
		{"literal mode", `{"items":[]}`, false, false, 2, 0},
		{"secret difference", `{"items":[null,null,null,null]}`, true, true, 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "GET http://example.test/before\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$.items\" count\nGET http://example.test/after\nHTTP 200\n[Asserts]\njsonpath \"$.items\" count == {{baseline}}\nGET http://example.test/end"
			calls := 0
			fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				body := tc.body
				if calls == 1 && tc.code != 2 {
					body = `{"items":["private-order",null,null]}`
				}
				return runner.Response{Status: 200, Body: []byte(body)}, nil
			})
			args := []string{"test", writeScenario(t, source)}
			if tc.enabled {
				args = append(args, "--render-asserts")
			}
			if tc.secret {
				args = append(args, "--secret", "baseline")
			}
			code, out, stderr := execute(t, context.Background(), fake, args...)
			if code != tc.code || calls != tc.calls || strings.Contains(out+stderr, "private-order") {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
			if tc.secret && (!strings.Contains(out, "esperado [oculto]") || !strings.Contains(out, "recebido [oculto]")) {
				t.Fatalf("unsafe comparison: %s", out)
			}
			if tc.name == "invalid capture" && (!strings.Contains(stderr, `consulta "$.items" count`) || !strings.Contains(stderr, ":4:") || !strings.Contains(stderr, "exige array")) {
				t.Fatalf("diagnostic=%s", stderr)
			}
		})
	}
}
