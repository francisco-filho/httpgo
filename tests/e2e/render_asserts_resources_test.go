package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRenderAssertsFinalAndIncompleteByBinary(t *testing.T) {
	for _, mode := range []string{"final", "incomplete", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
					return
				}
				if mode != "final" {
					w.Header().Set("Content-Length", "100")
				}
				w.Header().Set("X", "final")
				_, _ = io.WriteString(w, `"final"`)
				if mode == "timeout" {
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			source := "GET {{base}}/start\nHTTP 200\n[Asserts]\nheader \"X\" == \"{{value}}\"\njsonpath \"$\" == \"{{value}}\"\nGET {{base}}/next"
			args := []string{"test", scenario(t, source), "--render-asserts", "--var", "base=" + server.URL}
			if mode == "final" {
				args = append(args, "--var", "value=final")
			}
			if mode == "timeout" {
				args = append(args, "--config", configFile(t, t.TempDir(), "timeout.yaml", "version: 1\ndefaults: {timeout: 0.15}"))
			}
			code, out, stderr := run(t, args...)
			wantCode, wantHits := 2, int32(2)
			if mode == "final" {
				wantCode, wantHits = 0, 3
			}
			if code != wantCode || hits.Load() != wantHits || strings.Contains(stderr, "variável ausente") || strings.Contains(stderr, "avaliação") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}

func TestRenderAssertsSuiteIsolationByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, `9007199254740993`)
	}))
	defer server.Close()
	first := scenario(t, "GET {{base}}/capture\nHTTP 200\n[Captures]\nsaved: jsonpath \"$\"\nGET {{base}}/check\nHTTP 200\n[Asserts]\njsonpath \"$\" == {{saved}}")
	second := scenario(t, "GET {{base}}/isolated\nHTTP 200\n[Asserts]\njsonpath \"$\" == {{saved}}")
	code, out, stderr := run(t, "test", first, second, "--render-asserts", "--var", "base="+server.URL)
	if code != 2 || hits.Load() != 3 || !strings.Contains(stderr, "variável ausente") || !strings.Contains(out, "sucessos=1") || !strings.Contains(out, "erros=1") {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}
