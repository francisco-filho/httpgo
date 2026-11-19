package e2e

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Exercise US-24 assertions and US-25 captures in the same response after rebase.
func TestHeaderMatchesBeforeRegexCaptureByBinary(t *testing.T) {
	for _, mode := range []string{"success", "assertion mismatch", "late invalid assertion"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/session" {
					cookie := "refresh=fiction-integrated-token; Path=/; HttpOnly"
					if mode != "assertion mismatch" {
						cookie += "; Secure"
					}
					w.Header().Set("Set-Cookie", cookie)
					w.Header().Set("Content-Type", "text/plain; charset=unknown")
					_, _ = w.Write([]byte{0xff}) // Header assertion/capture must not decode body.
					return
				}
				if r.URL.Path != "/next" || r.Header.Get("Cookie") != "refresh=fiction-integrated-token" {
					t.Error("wrong request or cookie attributes included")
				}
			}))
			defer server.Close()
			source := "GET {{base}}/session\nHTTP 200\n" +
				"[Captures]\nrefresh_token: header \"Set-Cookie\" regex \"refresh=([^;]+)\"\n" +
				"[Asserts]\nheader \"Set-Cookie\" matches \"^refresh=[^;]+; Path=/; HttpOnly; Secure$\"\n" +
				"GET {{base}}/next\nCookie: refresh={{refresh_token}}\nHTTP 200"
			if mode == "late invalid assertion" {
				source += "\n[Asserts]\nheader \"Set-Cookie\" matches \"private[\""
			}
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL)
			wantCode, wantHits := 0, int32(2)
			switch mode {
			case "assertion mismatch":
				wantCode, wantHits = 1, 1
			case "late invalid assertion":
				wantCode, wantHits = 2, 0
			}
			if code != wantCode || hits.Load() != wantHits || strings.Contains(out+stderr, "fiction-integrated-token") || strings.Contains(out+stderr, "private[") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
}
