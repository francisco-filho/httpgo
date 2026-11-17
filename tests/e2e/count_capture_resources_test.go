package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCountCaptureCompleteFinalResponse(t *testing.T) {
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
					body, err := io.ReadAll(r.Body)
					if err != nil || string(body) != `{"count":3}` {
						t.Errorf("body=%s err=%v", body, err)
					}
					return
				}
				if mode != "final" {
					w.Header().Set("Content-Length", "100")
				}
				_, _ = io.WriteString(w, `[null,null,null]`)
				if mode == "timeout" {
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			source := "GET {{base}}/start\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$\" count\nPOST {{base}}/next\n{\"count\":{{baseline}}}\nHTTP 200"
			args := []string{"test", scenario(t, source), "--var", "base=" + server.URL, "--secret", "baseline"}
			if mode == "timeout" {
				args = append(args, "--config", configFile(t, t.TempDir(), "timeout.yaml", "version: 1\ndefaults: {timeout: 0.15}"))
			}
			code, out, stderr := run(t, args...)
			wantCode, wantHits := 2, int32(2)
			if mode == "final" {
				wantCode, wantHits = 0, 3
			}
			if code != wantCode || hits.Load() != wantHits || strings.Contains(stderr, "captura") || strings.Contains(out+stderr, "[null") || mode != "final" && !strings.Contains(out, "concluídas=0") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}

func TestCountCaptureSuiteNoSharedBaseline(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); _, _ = io.WriteString(w, `[1,2,3]`) }))
	defer server.Close()
	dir := t.TempDir()
	configFile(t, dir, "01.hurl", "GET {{base}}/first\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$\" count\nGET {{base}}/check\nHTTP 200\n[Asserts]\njsonpath \"$\" count == {{baseline}}")
	configFile(t, dir, "02.hurl", "GET {{base}}/other\nHTTP 200\n[Asserts]\njsonpath \"$\" count == {{baseline}}\nGET {{base}}/never")
	code, out, stderr := run(t, "test", dir, "--render-asserts", "--secret", "baseline", "--var", "base="+server.URL)
	if code != 2 || hits.Load() != 3 || !strings.Contains(stderr, "variável ausente") || strings.Contains(out+stderr, "[1,2,3]") {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}
