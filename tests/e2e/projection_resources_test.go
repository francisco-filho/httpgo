package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProjectionFinalAndIncompleteByBinary(t *testing.T) {
	for _, capture := range []bool{false, true} {
		for _, mode := range []string{"final", "incomplete", "timeout"} {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
					_, _ = io.WriteString(w, `["intermediate"]`)
					return
				}
				if r.URL.Path == "/next" {
					data, _ := io.ReadAll(r.Body)
					if capture && string(data) != `["final","second"]` {
						t.Errorf("wrong final capture: %s", data)
					}
					return
				}
				if mode != "final" {
					w.Header().Set("Content-Length", "100")
				}
				_, _ = io.WriteString(w, `["final","second"]`)
				if mode == "timeout" {
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}
			}))
			lines := "[Asserts]\njsonpath \"$[*]\" contains \"final\""
			next := "GET {{base}}/next"
			if capture {
				lines = "[Captures]\nx: jsonpath \"$[*]\""
				next = "POST {{base}}/next\n{{x}}"
			}
			args := []string{"test", scenario(t, "GET {{base}}/start\nHTTP *\n"+lines+"\n"+next), "--var", "base=" + server.URL}
			if mode == "timeout" {
				args = append(args, "--config", configFile(t, t.TempDir(), "timeout.yaml", "version: 1\ndefaults: {timeout: 0.1}"))
			}
			code, out, stderr := run(t, args...)
			server.Close()
			wantCode, wantHits := 2, int32(2)
			if mode == "final" {
				wantCode, wantHits = 0, 3
			}
			if code != wantCode || hits.Load() != wantHits || strings.Contains(stderr, "erro de captura") || wantCode == 2 && !strings.Contains(out, "concluídas=0") {
				t.Fatalf("mode=%s capture=%v code=%d hits=%d out=%s err=%s", mode, capture, code, hits.Load(), out, stderr)
			}
		}
	}
}
