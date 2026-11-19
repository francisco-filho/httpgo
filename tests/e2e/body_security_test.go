package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBodySecretExpectedByBinary(t *testing.T) {
	for _, pending := range []bool{false, true} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if r.URL.Path == "/login" {
				_, _ = io.WriteString(w, `{"token":"captured-private"}`)
				return
			}
			_, _ = io.WriteString(w, "actual-private body-private captured-private")
		}))
		source := "GET {{base}}/assert\nHTTP *\n[Asserts]\nbody contains \"captured-private\"\nGET {{base}}/never"
		if pending {
			source = strings.Replace(source, "\n[Asserts]", "\n[Captures]\ntoken: regex \"(.*)\"\n[Asserts]", 1)
		} else {
			// Known captured literal is used as the prohibited text in a later entry.
			source = "GET {{base}}/login\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\n" + strings.Replace(source, "body contains", "body not contains", 1)
		}
		if pending {
			source = strings.Replace(source, "body contains", "body not contains", 1)
		}
		code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL, "--secret", "token")
		server.Close()
		wantHits := int32(2)
		if pending {
			wantHits = 1
		}
		if code != 1 || hits.Load() != wantHits || !strings.Contains(stderr, "[oculto]") || strings.Contains(stderr, "erro de captura") {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
		for _, forbidden := range []string{"captured-private", "actual-private", "body-private"} {
			if strings.Contains(out+stderr, forbidden) {
				t.Fatalf("leaked %q: %s %s", forbidden, out, stderr)
			}
		}
	}
}

func TestBodyFinalResponseAndIncompleteByBinary(t *testing.T) {
	for _, mode := range []string{"final", "intermediate", "incomplete", "timeout"} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if r.URL.Path == "/start" {
				w.Header().Set("Content-Type", "text/plain; charset=unknown")
				w.Header().Set("Location", "/final")
				w.WriteHeader(302)
				_, _ = io.WriteString(w, "passwordHash\xff")
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=iso-8859-1")
			if mode == "incomplete" || mode == "timeout" {
				w.Header().Set("Content-Length", "100")
			}
			_, _ = io.WriteString(w, "customer a\xe7\xe3o body-private")
			if mode == "timeout" {
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}
		}))
		codeWant, hitsWant := 0, int32(3)
		expression := "body contains \"ação\"\nbody not contains \"passwordHash\""
		if mode == "intermediate" {
			expression = `body contains "passwordHash"`
			codeWant, hitsWant = 1, 2
		}
		if mode == "incomplete" || mode == "timeout" {
			codeWant, hitsWant = 2, 2
		}
		dir := t.TempDir()
		config := configFile(t, dir, "config.yaml", "version: 1\ndefaults: {timeout: 0.2}\n")
		code, out, stderr := run(t, "test", scenario(t, "GET {{base}}/start\nHTTP *\n[Asserts]\n"+expression+"\nGET {{base}}/never"), "--var", "base="+server.URL, "--config", config)
		server.Close()
		if code != codeWant || hits.Load() != hitsWant || codeWant == 2 && !strings.Contains(out, "concluídas=0") || strings.Contains(out+stderr, "body-private") {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
	}
}
