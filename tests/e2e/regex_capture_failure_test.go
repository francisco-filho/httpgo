package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRegexCaptureFinalAndFailureByBinary(t *testing.T) {
	for _, origin := range []string{"body", "header"} {
		for _, mode := range []string{"success", "intermediate only", "incomplete", "mixed", "pending", "later status"} {
			t.Run(origin+"/"+mode, func(t *testing.T) {
				var hits atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits.Add(1)
					switch r.URL.Path {
					case "/start":
						w.Header().Set("Set-Cookie", "refresh=intermediate; HttpOnly")
						w.Header().Set("Location", "/final")
						w.WriteHeader(307)
						_, _ = io.WriteString(w, "code=intermediate")
					case "/final":
						if mode == "incomplete" {
							w.Header().Set("Content-Length", "200")
						}
						if mode == "intermediate only" {
							_, _ = io.WriteString(w, "no code")
							return
						}
						w.Header().Set("Set-Cookie", "refresh=fiction-final-group; Path=/; HttpOnly")
						_, _ = io.WriteString(w, "code=fiction-final-group")
					case "/next":
						if r.Header.Get("X-Value") != "prefix-fiction-final-group" || r.Header.Get("Cookie") != "" {
							t.Error("wrong group or implicit cookie")
						}
						if mode == "later status" {
							w.WriteHeader(503)
						}
					default:
						t.Error("unexpected request")
					}
				}))
				defer server.Close()
				decl := `x: regex "code=(.+)"`
				if origin == "header" {
					decl = `x: header "Set-Cookie" regex "refresh=([^;]+)"`
				}
				source := "GET {{base}}/start\nHTTP 200\n[Captures]\n" + decl + "\n"
				if mode == "mixed" {
					source += "bad: header \"Missing\"\n"
				}
				if mode == "pending" {
					source += "[Asserts]\nheader \"Other\" == \"pending-private-operand\"\n"
				}
				source += "GET {{base}}/next\nX-Value: prefix-{{x}}\nHTTP 200"
				code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL, "--secret", "x")
				wantCode, wantHits := 2, int32(2)
				switch mode {
				case "success":
					wantCode, wantHits = 0, 3
				case "pending":
					wantCode = 1
				case "later status":
					wantCode, wantHits = 1, 3
				}
				if code != wantCode || hits.Load() != wantHits || strings.Contains(out+stderr, "fiction-final-group") || strings.Contains(out+stderr, "pending-private-operand") {
					t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
				}
				if mode == "incomplete" && !strings.Contains(out, "concluídas=0") {
					t.Fatal(out)
				}
			})
		}
	}
}
