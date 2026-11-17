package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-487: published demonstration, real HTTP and binary, success and mismatch.
func TestRegexAssertionSignupByBinary(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "approved", true: "mismatch"}[mismatch], func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/message/123":
					_, _ = io.WriteString(w, "Seu código de verificação é: 123456")
				case "/verify":
					body, err := io.ReadAll(r.Body)
					if err != nil || r.Method != "POST" || string(body) != `{"code":"123456"}` {
						t.Errorf("method=%s body=%s err=%v", r.Method, body, err)
					}
				default:
					t.Errorf("path=%s", r.URL.Path)
				}
			}))
			defer server.Close()
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "scenarios", "scenario_regex_assertion_signup.hurl"))
			if err != nil {
				t.Fatal(err)
			}
			if mismatch {
				data = []byte(strings.Replace(string(data), `^[0-9]{6}$`, `^[0-9]{7}$`, 1))
			}
			code, out, stderr := run(t, "test", scenario(t, string(data)), "--var", "mailbox_url="+server.URL, "--var", "base_url="+server.URL, "--secret", "signup_code")
			wantCode, wantHits := 0, int32(2)
			if mismatch {
				wantCode, wantHits = 1, 1
			}
			if code != wantCode || hits.Load() != wantHits || strings.Contains(out+stderr, "123456") || strings.Contains(out+stderr, "Seu código") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
			if mismatch && (!strings.Contains(stderr, "matches") || !strings.Contains(stderr, "grupo presente") || !strings.Contains(stderr, "[oculto]")) {
				t.Fatal(stderr)
			}
		})
	}
}

func TestRegexAssertionPreflightByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, decl := range []string{`regex "private[" exists`, `regex "(?=private)" exists`, `regex "\d" exists`, `regex "(x)" == x`, `regex "(x)" exists extra`, `regex "(x)" matches "["`, `regex "(x)" regex "x" exists`} {
		code, out, stderr := run(t, "test", scenario(t, "GET {{base}}/first\nHTTP 200\nGET {{base}}/late\nHTTP 200\n[Asserts]\n"+decl), "--var", "base="+server.URL)
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":6:") || strings.Contains(stderr, "private") {
			t.Fatalf("decl=%s code=%d hits=%d out=%q err=%q", decl, code, hits.Load(), out, stderr)
		}
	}
}
