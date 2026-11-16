package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBodyByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, body, contentType, expression string
		code                                int
	}{
		{"demo", "customer", "text/plain", "body contains \"customer\"\nbody not contains \"passwordHash\"\nbody not contains \"trackingToken\"", 0},
		{"forbidden", "customer passwordHash body-private", "text/plain", `body not contains "passwordHash"`, 1},
		{"non JSON", "ação 雪", "text/plain; charset=utf-8", `body contains "ação 雪"`, 0},
		{"latin1", "a\xe7\xe3o", "text/plain; charset=ISO-8859-1", `body contains "ação"`, 0},
		{"replacement valid", "�", "text/plain", `body contains "�"`, 0},
		{"literal JSON", "{\"x\": \"\\u0061\"}", "application/json", `body contains "\\u0061"`, 0},
		{"case", "Customer", "text/plain", `body contains "customer"`, 1},
		{"whitespace", "a  b", "text/plain", `body contains "a b"`, 1},
		{"unicode", "e\u0301", "text/plain", `body contains "é"`, 1},
		{"newline", "a\nb", "text/plain", `body contains "a\nb"`, 0},
		{"empty", "", "text/plain", `body contains ""`, 0},
		{"empty not empty", "", "text/plain", `body not contains ""`, 1},
		{"empty not text", "", "text/plain", `body not contains "x"`, 0},
		{"empty contains text", "", "text/plain", `body contains "x"`, 1},
		{"literal placeholders", "{{defined}} {{missing}}", "text/plain", `body contains "{{defined}} {{missing}}"`, 0},
		{"unknown", "", "text/plain; charset=private-label", `body not contains "x"`, 2},
		{"invalid", "\xffbody-private", "text/plain; charset=utf-8", `body not contains "x"`, 2},
		{"truncated", "\xe2\x82", "text/plain; charset=utf-8", `body contains ""`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.Header().Set("Content-Type", tc.contentType)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "GET {{base}}/first\nHTTP 200\n[Asserts]\n  "+tc.expression+"\nGET {{base}}/next"), "--var", "base="+server.URL, "--var", "defined=resolved")
			wantHits := int32(1)
			if tc.code == 0 {
				wantHits = 2
			}
			if code != tc.code || hits.Load() != wantHits {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			if tc.code != 0 && !strings.Contains(stderr, `":4:3:`) {
				t.Fatal(stderr)
			}
			if strings.Contains(out+stderr, "body-private") || strings.Contains(out+stderr, "private-label") {
				t.Fatalf("unsafe: %s %s", out, stderr)
			}
		})
	}
}

func TestBodyInvalidLateByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, name := range []string{"unsupported_assert_body_operator.hurl", "invalid_assert_body_late.hurl"} {
		data, err := os.ReadFile("../../testdata/scenarios/" + name)
		if err != nil {
			t.Fatal(err)
		}
		code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll(string(data), "http://localhost:8080", server.URL)))
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || stderr == "" {
			t.Fatalf("fixture=%s code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
	for _, suffix := range []string{
		"[Asserts]\nbody contains \"x\"",
		"HTTP *\n[Asserts]\nbody contains unquoted",
		"HTTP *\n[Asserts]\nbody contains \"x\" \"extra\"",
		"HTTP *\n[Asserts]\nbody matches \"x\"",
		"HTTP *\n[Asserts]\nbody not exists",
	} {
		code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/first\nHTTP 200\nGET "+server.URL+"/late\n"+suffix))
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || stderr == "" {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
	}
}
