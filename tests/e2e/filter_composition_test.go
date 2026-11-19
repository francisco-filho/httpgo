package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-347–351: nested entities retain association and plural captures retain type.
func TestFilterNestedCaptureByBinary(t *testing.T) {
	const catalog = `{"groups":[{"products":[{"slug":"pizza","active":false,"ids":[9223372036854775809]},{"slug":"juice","active":true,"ids":[0]}]},{"products":[{"slug":"pizza","active":true,"ids":[9007199254740993]}]}]}`
	for _, next := range []string{"POST {{base_url}}/echo\n{{ids}}\nHTTP 204", "GET {{base_url}}/{{ids}}", "GET {{base_url}}/echo\nX-Ids: {{ids}}"} {
		t.Run(next, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/echo" {
					body, err := io.ReadAll(r.Body)
					if err != nil || string(body) != `[9223372036854775809,9007199254740993]` {
						t.Errorf("body=%s err=%v", body, err)
					}
					w.WriteHeader(204)
					return
				}
				_, _ = io.WriteString(w, catalog)
			}))
			defer server.Close()
			source := "GET {{base_url}}/products\nHTTP *\n[Asserts]\njsonpath \"$.groups[*].products[?(@.slug=='pizza')].active\" == [false,true]\njsonpath \"$.groups[*].products[?(@.slug=='pizza')].ids[*]\" count == 2\n[Captures]\nids: jsonpath \"$.groups[*].products[?(@.slug=='pizza')].ids[*]\"\n" + next
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base_url="+server.URL)
			wantCode, wantHits := 2, int32(1)
			if strings.HasPrefix(next, "POST") {
				wantCode, wantHits = 0, 2
			}
			if code != wantCode || hits.Load() != wantHits || code == 0 && stderr != "" {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}
