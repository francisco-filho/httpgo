package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHeaderMatchesFinalAndIncompleteByBinary(t *testing.T) {
	for _, mode := range []string{"final", "intermediate only", "incomplete", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("X-Test", "intermediate")
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
					return
				}
				if r.URL.Path == "/next" {
					if r.Header.Get("If-None-Match") != "captured" {
						t.Error("capture not published")
					}
					return
				}
				if mode != "intermediate only" {
					w.Header().Set("X-Test", "final")
				}
				w.Header().Set("ETag", "captured")
				if mode == "incomplete" || mode == "timeout" {
					w.Header().Set("Content-Length", "100")
				}
				_, _ = io.WriteString(w, "not JSON")
				if mode == "timeout" {
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			pattern := "^final$"
			if mode == "intermediate only" {
				pattern = "^intermediate$"
			}
			source := "GET {{base}}/start\nHTTP *\n[Asserts]\nheader \"X-Test\" matches \"" + pattern + "\"\n[Captures]\nx: header \"ETag\"\nGET {{base}}/next\nIf-None-Match: {{x}}"
			args := []string{"test", scenario(t, source), "--var", "base=" + server.URL}
			if mode == "timeout" {
				args = append(args, "--config", configFile(t, t.TempDir(), "timeout.yaml", "version: 1\ndefaults: {timeout: 0.15}"))
			}
			code, out, stderr := run(t, args...)
			wantCode, wantHits := 2, int32(2)
			if mode == "final" {
				wantCode, wantHits = 0, 3
			}
			if mode == "intermediate only" {
				wantCode = 1
			}
			if code != wantCode || hits.Load() != wantHits || strings.Contains(stderr, "erro de captura") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			if code == 2 && !strings.Contains(out, "concluídas=0") {
				t.Fatal(out)
			}
		})
	}
}

func TestHeaderMatchesPendingSecretByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("X-Test", "actual-private")
		_, _ = io.WriteString(w, `{"unused":"body-private"}`)
	}))
	defer server.Close()
	source := "GET {{base}}/first\nHTTP 200\n[Asserts]\nheader \"X-Test\" matches \"expected-private\"\n[Captures]\ntoken: jsonpath \"$.missing.bad\"\nGET {{base}}/never"
	code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL, "--secret", "token")
	if code != 1 || hits.Load() != 1 || strings.Contains(out+stderr, "private") || strings.Contains(stderr, "erro de captura") || !strings.Contains(stderr, "[oculto]") {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}
