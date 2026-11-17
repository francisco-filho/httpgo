package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-428: real binary evaluates only the complete final response.
func TestJSONInequalityFinalResponse(t *testing.T) {
	for _, mode := range []string{"final", "incomplete", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
					_, _ = io.WriteString(w, `"old-private"`)
					return
				}
				if r.URL.Path == "/next" {
					if r.Header.Get("Authorization") != "Bearer new-private" {
						t.Error("wrong capture")
					}
					return
				}
				if mode != "final" {
					w.Header().Set("Content-Length", "100")
				}
				_, _ = io.WriteString(w, `"new-private"`)
				if mode == "timeout" {
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			source := "GET {{base}}/start\nHTTP 200\n[Asserts]\njsonpath \"$\" != \"old-private\"\n[Captures]\nnew: jsonpath \"$\"\nGET {{base}}/next\nAuthorization: Bearer {{new}}"
			args := []string{"test", scenario(t, source), "--var", "base=" + server.URL, "--secret", "new"}
			if mode == "timeout" {
				args = append(args, "--config", configFile(t, t.TempDir(), "timeout.yaml", "version: 1\ndefaults: {timeout: 0.15}"))
			}
			code, out, stderr := run(t, args...)
			wantCode, wantHits := 2, int32(2)
			if mode == "final" {
				wantCode, wantHits = 0, 3
			}
			if code != wantCode || hits.Load() != wantHits || strings.Contains(stderr, "avaliação") || strings.Contains(stderr, "captura") || mode != "final" && !strings.Contains(out, "concluídas=0") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			if strings.Contains(out+stderr, "old-private") || strings.Contains(out+stderr, "new-private") {
				t.Fatal("unsafe output")
			}
		})
	}
}
