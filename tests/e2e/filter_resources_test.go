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

// GO-359: the real binary rejects incomplete valid JSON and interrupted bodies,
// even if the filter would already match the received prefix.
func TestFilterFinalAndIncompleteByBinary(t *testing.T) {
	for _, capture := range []bool{false, true} {
		for _, mode := range []string{"final", "incomplete", "timeout"} {
			t.Run(fmt.Sprintf("%s/capture=%v", mode, capture), func(t *testing.T) {
				var hits atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits.Add(1)
					if r.URL.Path == "/start" {
						w.Header().Set("Location", "/final")
						w.WriteHeader(307)
						_, _ = io.WriteString(w, `[{"key":"x","id":"intermediate"}]`)
						return
					}
					if r.URL.Path == "/next" {
						body, _ := io.ReadAll(r.Body)
						if capture && string(body) != `["final","second"]` {
							t.Errorf("capture=%s", body)
						}
						return
					}
					if mode != "final" {
						w.Header().Set("Content-Length", "200")
					}
					_, _ = io.WriteString(w, `[{"key":"x","id":"final"},{"key":"x","id":"second"}]`)
					if mode == "timeout" {
						w.(http.Flusher).Flush()
						<-r.Context().Done()
					}
				}))
				defer server.Close()
				lines := "[Asserts]\njsonpath \"$[?(@.key=='x')].id\" == [\"final\",\"second\"]"
				next := "GET {{base}}/next"
				if capture {
					lines = "[Captures]\nx: jsonpath \"$[?(@.key=='x')].id\""
					next = "POST {{base}}/next\n{{x}}"
				}
				args := []string{"test", scenario(t, "GET {{base}}/start\nHTTP *\n"+lines+"\n"+next), "--var", "base=" + server.URL}
				if mode == "timeout" {
					args = append(args, "--config", configFile(t, t.TempDir(), "timeout.yaml", "version: 1\ndefaults: {timeout: 0.1}"))
				}
				code, out, stderr := run(t, args...)
				wantCode, wantHits := 2, int32(2)
				if mode == "final" {
					wantCode, wantHits = 0, 3
				}
				if code != wantCode || hits.Load() != wantHits || strings.Contains(stderr, "erro de captura") || wantCode == 2 && !strings.Contains(out, "concluídas=0") {
					t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
				}
			})
		}
	}
}
