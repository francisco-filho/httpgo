package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMembershipFinalAndIncompleteByBinary(t *testing.T) {
	for _, predicate := range []string{`header "X" not contains "forbidden"`, `jsonpath "$.items" not contains "forbidden"`} {
		for _, mode := range []string{"final", "forbidden final", "incomplete", "timeout"} {
			t.Run(predicate+mode, func(t *testing.T) {
				var hits atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits.Add(1)
					if r.URL.Path == "/start" {
						w.Header().Set("X", "forbidden")
						w.Header().Set("Location", "/final")
						w.WriteHeader(307)
						_, _ = io.WriteString(w, `{"items":["forbidden"]}`)
						return
					}
					if r.URL.Path == "/next" {
						if r.Header.Get("If-None-Match") != "final" {
							t.Error("capture not published")
						}
						return
					}
					w.Header().Set("ETag", "final")
					body := `{"items":[]}`
					if mode == "forbidden final" {
						w.Header().Set("X", "forbidden")
						body = `{"items":["forbidden"]}`
					}
					if mode == "incomplete" || mode == "timeout" {
						w.Header().Set("Content-Length", "100")
					}
					_, _ = io.WriteString(w, body)
					if mode == "timeout" {
						w.(http.Flusher).Flush()
						<-r.Context().Done()
					}
				}))
				defer server.Close()
				source := "GET {{base}}/start\nHTTP *\n[Asserts]\n" + predicate + "\n[Captures]\nx: header \"ETag\"\nGET {{base}}/next\nIf-None-Match: {{x}}"
				args := []string{"test", scenario(t, source), "--var", "base=" + server.URL}
				if mode == "timeout" {
					config := configFile(t, t.TempDir(), "timeout.yaml", "version: 1\ndefaults: {timeout: 0.15}")
					args = append(args, "--config", config)
				}
				code, out, stderr := run(t, args...)
				wantCode, wantHits := 2, int32(2)
				if mode == "final" {
					wantCode, wantHits = 0, 3
				}
				if mode == "forbidden final" {
					wantCode = 1
				}
				if code != wantCode || hits.Load() != wantHits || strings.Contains(stderr, "erro de captura") {
					t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
				}
				if wantCode == 2 && !strings.Contains(out, "concluídas=0") {
					t.Fatal(out)
				}
			})
		}
	}
}
