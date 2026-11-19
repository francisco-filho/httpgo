package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHeaderCaptureFinalAndIncompleteByBinary(t *testing.T) {
	for _, mode := range []string{"final", "missing final", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/start":
					w.Header().Set("ETag", `"intermediate"`)
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
				case "/final":
					if mode != "missing final" {
						w.Header().Set("ETag", `"final"`)
					}
					w.Header().Set("Set-Cookie", "session=private-header")
					if mode == "incomplete" {
						w.Header().Set("Content-Length", "100")
					}
					_, _ = io.WriteString(w, "not JSON")
				case "/next":
					if r.Header.Get("If-None-Match") != `"final"` || r.Header.Get("Cookie") != "" {
						t.Error("intermediate header or implicit cookie")
					}
				default:
					t.Error("unexpected request")
				}
			}))
			defer server.Close()
			source := "GET {{base}}/start\nHTTP 200\n[Captures]\nx: header \"ETag\"\nGET {{base}}/next\nIf-None-Match: {{x}}\nHTTP 200"
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL)
			wantCode, wantHits := 2, int32(2)
			if mode == "final" {
				wantCode, wantHits = 0, 3
			}
			if code != wantCode || hits.Load() != wantHits || strings.Contains(out+stderr, "private-header") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
			if mode == "incomplete" && !strings.Contains(out, "concluídas=0") {
				t.Fatal(out)
			}
		})
	}
}

func TestSensitiveHeaderCaptureLaterFailureByBinary(t *testing.T) {
	for _, mode := range []string{"status", "assertion", "missing", "render"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/first" {
					w.Header().Set("Set-Cookie", "session=private-cookie; Path=/; HttpOnly")
					return
				}
				if r.Header.Get("X-Value") != "prefix-session=private-cookie; Path=/; HttpOnly" || r.Header.Get("Cookie") != "" {
					t.Error("operational value changed or implicit cookie")
				}
				w.WriteHeader(503)
			}))
			defer server.Close()
			source := "GET {{base}}/first\nHTTP 200\n[Captures]\nx: header \"Set-Cookie\"\n"
			if mode == "missing" {
				source += "missing: header \"Absent\"\n"
			}
			if mode == "assertion" {
				source += "[Asserts]\nheader \"Other\" == \"pending-private-operand\"\n"
			}
			if mode == "render" {
				source += "GET {{base}}/{{absent}}\n"
			} else {
				source += "GET {{base}}/next\nX-Value: prefix-{{x}}\nHTTP 200\n"
			}
			source += "GET {{base}}/last"
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL)
			wantCode, wantHits := 1, int32(2)
			if mode != "status" {
				wantHits = 1
			}
			if mode == "missing" || mode == "render" {
				wantCode = 2
			}
			if code != wantCode || hits.Load() != wantHits || strings.Contains(out+stderr, "private-cookie") || strings.Contains(out+stderr, "pending-private-operand") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
}
