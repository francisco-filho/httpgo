package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestJSONTypeFinalAndIncompleteByBinary(t *testing.T) {
	for _, mode := range []string{"final", "incomplete", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
					_, _ = io.WriteString(w, `[null]`)
					return
				}
				if r.URL.Path == "/next" {
					body, _ := io.ReadAll(r.Body)
					if string(body) != `[]` {
						t.Errorf("capture=%s", body)
					}
					return
				}
				if mode != "final" {
					w.Header().Set("Content-Length", "200")
				}
				_, _ = io.WriteString(w, `[]`)
				if mode == "timeout" {
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			args := []string{"test", scenario(t, "GET {{base}}/start\nHTTP *\n[Asserts]\njsonpath \"$\" isEmpty\n[Captures]\nx: jsonpath \"$\"\nPOST {{base}}/next\n{{x}}"), "--var", "base=" + server.URL}
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
