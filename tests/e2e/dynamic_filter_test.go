package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

// GO-396,401–404: exact prior capture, entity association, opt-in and full-file validation.
func TestDynamicFilterDemonstration(t *testing.T) {
	for _, tc := range []struct {
		name, status, operand string
		enabled               bool
		code, hits            int
	}{
		{"selected", "PLACED", `{{order_number}}`, true, 0, 3},
		{"cancelled", "CANCELLED", `{{order_number}}`, true, 1, 2},
		{"absent", "PLACED", `12`, true, 1, 2},
		{"missing", "PLACED", `{{missing}}`, true, 2, 2},
		{"no opt-in", "PLACED", `{{order_number}}`, false, 2, 0},
		{"quoted no opt-in", "PLACED", `'{{order_number}}'`, false, 2, 0},
		{"static late", "PLACED", `{{broken`, true, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/first":
					_, _ = io.WriteString(w, `{"number":9007199254740993}`)
				case "/orders":
					_, _ = fmt.Fprintf(w, `{"items":[{"number":9007199254740992,"status":"PLACED","id":"wrong"},{"number":9007199254740993,"status":%q,"id":"selected"}]}`, tc.status)
				case "/selected":
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer server.Close()
			path := `$.items[?(@.number==` + tc.operand + `)]`
			source := "GET {{base}}/first\nHTTP 200\n[Captures]\norder_number: jsonpath \"$.number\"\nGET {{base}}/orders\nHTTP 200\n[Asserts]\njsonpath " + strconv.Quote(path+".status") + " == \"PLACED\"\n[Captures]\nselected_id: jsonpath " + strconv.Quote(path+".id") + "\nGET {{base}}/{{selected_id}}"
			args := []string{"test", scenario(t, source), "--var", "base=" + server.URL}
			if tc.enabled {
				args = append(args, "--render-asserts")
			}
			code, out, stderr := run(t, args...)
			if code != tc.code || int(hits.Load()) != tc.hits {
				t.Fatalf("code=%d hits=%d stdout=%s stderr=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}
