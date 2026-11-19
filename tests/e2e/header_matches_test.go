package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-272: a dynamic cookie is checked without publishing captures on failure.
func TestHeaderMatchesSessionByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, cookie string
		code         int
	}{
		{"session one", "session=alpha; Path=/; HttpOnly; Secure; SameSite=Lax", 0},
		{"session two", "session=beta-123; Path=/; HttpOnly; Secure; SameSite=Lax", 0},
		{"missing Secure", "session=alpha; Path=/; HttpOnly; SameSite=Lax", 1},
		{"wrong format", "other=alpha; Path=/; HttpOnly; Secure; SameSite=Lax", 1},
		{"absent", "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path != "/session" && r.URL.Path != "/captured" {
					t.Errorf("path=%q", r.URL.Path)
				}
				if tc.cookie != "" {
					w.Header().Set("Set-Cookie", tc.cookie)
				}
				_, _ = io.WriteString(w, `{"id":"captured"}`)
			}))
			defer server.Close()
			source := "GET {{base_url}}/session\nHTTP 200\n[Asserts]\nheader \"Set-Cookie\" matches \"^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$\"\n[Captures]\nid: jsonpath \"$.id\"\nGET {{base_url}}/{{id}}"
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base_url="+server.URL)
			wantHits := int32(2)
			if tc.code != 0 {
				wantHits = 1
			}
			if code != tc.code || hits.Load() != wantHits || strings.Contains(out+stderr, "session=") || strings.Contains(out+stderr, "SameSite=Lax") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
			if code == 1 && (!strings.Contains(stderr, "matches") || !strings.Contains(stderr, "[oculto]") || !strings.Contains(out, "não_iniciadas=1")) {
				t.Fatalf("out=%q err=%q", out, stderr)
			}
		})
	}
}

func TestHeaderMatchesInvalidLateByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, pattern := range []string{`"private["`, `"(?=private)"`, `"(private)\\1"`, `"\x41private"`, `/private/`, `"private" extra`} {
		for _, newline := range []string{"\n", "\r\n"} {
			source := "GET {{base}}/first\nHTTP 200\nGET {{base}}/late\nHTTP *\n[Asserts]\n  header \"Set-Cookie\" matches " + pattern
			code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll(source, "\n", newline)), "--var", "base="+server.URL)
			if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":6:3:") || strings.Contains(stderr, "private") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		}
	}
}
