package e2e

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCaptureFinalRedirectResponseByBinary(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/login":
					w.Header().Set("Location", "/final")
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"token":"wrong-token"}`)
				case "/final":
					want := "GET"
					if status == 307 || status == 308 {
						want = "POST"
					}
					body, _ := io.ReadAll(r.Body)
					if r.Method != want || want == "GET" && len(body) != 0 || want == "POST" && string(body) != `{"login":true}` {
						t.Errorf("method=%s body=%s", r.Method, body)
					}
					_, _ = io.WriteString(w, `{"token":"final-token"}`)
				case "/protected":
					if r.Header.Get("Authorization") != "Bearer final-token" {
						t.Error("intermediate response captured")
					}
				default:
					t.Error("unexpected request")
				}
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "POST {{base}}/login\n{\"login\":true}\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\nGET {{base}}/protected\nAuthorization: Bearer {{token}}\nHTTP 200"), "--var", "base="+server.URL, "--secret", "token")
			if code != 0 || stderr != "" || hits.Load() != 3 || !strings.Contains(out, "validadas=2") || strings.Contains(out, "final-token") || strings.Contains(out, "wrong-token") {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
		})
	}
}

func TestCaptureTransportErrorsByBinary(t *testing.T) {
	for _, mode := range []string{"incomplete", "TLS", "cross origin", "no retry"} {
		t.Run(mode, func(t *testing.T) {
			var hits, next, targetHits atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { targetHits.Add(1) }))
			defer target.Close()
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/next" {
					next.Add(1)
					return
				}
				hits.Add(1)
				switch mode {
				case "incomplete":
					w.Header().Set("Content-Length", "100")
					_, _ = io.WriteString(w, `{"token":"private-token"}`)
				case "cross origin":
					w.Header().Set("Location", target.URL)
					w.WriteHeader(307)
				case "no retry":
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
				}
			}))
			server.Config.ErrorLog = log.New(io.Discard, "", 0)
			if mode == "TLS" {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "POST {{base}}/login\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\nGET {{base}}/next"), "--var", "base="+server.URL, "--secret", "token")
			wantHits := int32(1)
			if mode == "TLS" {
				wantHits = 0
			}
			if code != 2 || hits.Load() != wantHits || next.Load() != 0 || targetHits.Load() != 0 || !strings.Contains(out, "erro de transporte") || !strings.Contains(out, "concluídas=0") || strings.Contains(out+stderr, "private-token") {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
		})
	}
}
