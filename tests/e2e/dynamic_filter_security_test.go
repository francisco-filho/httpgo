package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-409: the real binary protects dynamically captured secrets and substituted
// operands in normal diagnostics, not only when the output writer fails.
func TestDynamicFilterSecretStreams(t *testing.T) {
	for _, mode := range []string{"mismatch", "type", "missing", "capture"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/capture" {
					value := `"fake-private"`
					if mode == "type" {
						value = `{"key":"fake-private"}`
					}
					_, _ = io.WriteString(w, value)
					return
				}
				if r.URL.Path != "/items" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				_, _ = io.WriteString(w, `[{"key":"Order fake-private","id":"derived-private"}]`)
			}))
			defer server.Close()
			operand := "key"
			if mode == "missing" {
				operand = "missing"
			}
			declaration := "[Asserts]\njsonpath \"$[?(@.key=='Order {{" + operand + "}}')].id\" == \"not-selected\""
			if mode == "capture" {
				declaration = "[Captures]\nselected: jsonpath \"$[?(@.key=='Order {{key}}')].id\"\nlate: jsonpath \"$[?(@.key=={{missing}})]\""
			}
			source := "GET {{base}}/capture\nHTTP 200\n[Captures]\nkey: jsonpath \"$\"\nGET {{base}}/items\nHTTP 200\n" + declaration + "\nGET {{base}}/never"
			code, out, stderr := run(t, "test", scenario(t, source), "--render-asserts", "--secret", "key", "--var", "base="+server.URL)
			want := 2
			if mode == "mismatch" {
				want = 1
			}
			if code != want || hits.Load() != 2 {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			for _, bad := range []string{"fake-private", "derived-private", "{{key}}", "not-selected"} {
				if strings.Contains(out+stderr, bad) {
					t.Fatalf("unsafe streams: %s %s", out, stderr)
				}
			}
		})
	}
}
