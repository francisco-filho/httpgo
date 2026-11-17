package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-377,379,385,387: the invocation selects parsing and execution together.
func TestJSONRenderAssertsDemonstration(t *testing.T) {
	for _, tc := range []struct {
		name, number, operand string
		enabled               bool
		code, hits            int
	}{
		{"exact", "9007199254740993", `{{order_number}}`, true, 0, 3},
		{"different", "9007199254740992", `{{order_number}}`, true, 1, 2},
		{"string different", "9007199254740993", `"{{order_number}}"`, true, 1, 2},
		{"missing", "9007199254740993", `{{missing}}`, true, 2, 2},
		{"literal rejects bare", "9007199254740993", `{{order_number}}`, false, 2, 0},
		{"static late", "9007199254740993", `{{broken`, true, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/orders":
					if r.Method != "POST" {
						t.Error("not POST")
					}
					_, _ = io.WriteString(w, `{"number":9007199254740993,"name":"Pizza"}`)
				case "/orders/9007199254740993":
					_, _ = fmt.Fprintf(w, `{"number":%s,"name":"Pizza"}`, tc.number)
				case "/next":
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer server.Close()
			source := "POST {{base}}/orders\nHTTP 200\n[Captures]\norder_number: jsonpath \"$.number\"\norder_name: jsonpath \"$.name\"\nGET {{base}}/orders/{{order_number}}\nHTTP 200\n[Asserts]\njsonpath \"$.number\" == " + tc.operand + "\njsonpath \"$.name\" == \"{{order_name}}\"\n[Captures]\nchecked: jsonpath \"$.number\"\nGET {{base}}/next"
			args := []string{"test", scenario(t, source), "--var", "base=" + server.URL}
			if tc.enabled {
				args = append(args, "--render-asserts")
			}
			code, out, stderr := run(t, args...)
			if code != tc.code || int(hits.Load()) != tc.hits {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			if tc.code == 2 && tc.hits > 0 && !strings.Contains(stderr, "erro de avaliação JSON") {
				t.Fatalf("wrong error: %s", stderr)
			}
		})
	}
}
