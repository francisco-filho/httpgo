package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRegexAssertionProfilesByBinary(t *testing.T) {
	for _, render := range []bool{false, true} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); _, _ = io.WriteString(w, "id=current") }))
		source := "GET {{base}}\nHTTP 200\n[Asserts]\nregex \"id=(.*)\" == \"{{id}}\"\n[Captures]\nid: regex \"id=(.*)\"\nGET {{base}}/{{id}}"
		args := []string{"test", scenario(t, source), "--var", "base=" + server.URL, "--var", "id=initial"}
		if render {
			args = append(args, "--render-asserts")
		}
		code, out, stderr := run(t, args...)
		server.Close()
		wantCode, wantHits := 1, int32(1)
		if render {
			wantCode, wantHits = 0, 2
		}
		if code != wantCode || hits.Load() != wantHits {
			t.Fatalf("render=%v code=%d hits=%d out=%q err=%q", render, code, hits.Load(), out, stderr)
		}
	}
}

func TestRegexAssertionPollingByBinary(t *testing.T) {
	for _, mode := range []string{"success", "error"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := hits.Add(1)
				if n == 3 {
					if r.URL.Path != "/accepted" {
						t.Error(r.URL.Path)
					}
					return
				}
				if r.URL.Path != "/old" {
					t.Error(r.URL.Path)
				}
				id, state := "rejected", "PENDING"
				if n == 2 {
					id, state = "accepted", "READY"
				}
				w.Header().Set("X-ID", id)
				_, _ = io.WriteString(w, "id="+id+";state="+state)
			}))
			defer server.Close()
			decl := `regex "state=(.*)" == "READY"`
			if mode == "error" {
				decl = `regex "state" not contains "x"`
			}
			source := "GET {{base}}/{{id}}\n[Options]\nretry: 2\nretry-interval: 0\nHTTP 200\n[Captures]\nid: header \"X-ID\"\n[Asserts]\nregex \"id=([^;]+)\" == \"{{id}}\"\n" + decl + "\nGET {{base}}/{{id}}"
			code, out, stderr := run(t, "test", scenario(t, source), "--render-asserts", "--var", "base="+server.URL, "--var", "id=old")
			wantCode, wantHits := 0, int32(3)
			if mode == "error" {
				wantCode, wantHits = 2, 1
			}
			if code != wantCode || hits.Load() != wantHits || mode == "success" && !strings.Contains(out, "tentativas=2") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
}
