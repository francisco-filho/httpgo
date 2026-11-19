package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-454: decode is lazy and cannot override an earlier status/header failure.
func TestProjectionInvalidJSONPrecedence(t *testing.T) {
	for _, body := range []string{"", "[", "[1,2] null"} {
		for _, tc := range []struct {
			status int
			lines  string
			code   int
		}{
			{503, "[Asserts]\njsonpath \"$[*]\" not exists", 1},
			{200, "[Asserts]\nheader \"X\" == \"required\"\njsonpath \"$[*]\" not exists", 1},
			{200, "[Asserts]\njsonpath \"$[*]\" not exists\nheader \"X\" == \"required\"", 2},
			{200, "[Asserts]\nbody contains \"\"\njsonpath \"$[*]\" not exists", 2},
			{200, "[Captures]\nids: jsonpath \"$[*]\"", 2},
		} {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, body)
			}))
			code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"\nHTTP 200\n"+tc.lines+"\nGET "+server.URL+"/never"))
			server.Close()
			if code != tc.code || hits.Load() != 1 || tc.code == 1 && strings.Contains(stderr, "JSON válido") {
				t.Fatalf("body=%q code=%d hits=%d out=%s err=%s", body, code, hits.Load(), out, stderr)
			}
		}
	}
}

// GO-451: no implicit first element or collection-to-text conversion.
func TestProjectionCollectionTextNeverSends(t *testing.T) {
	for _, next := range []string{"GET {{base}}/{{ids}}", "GET {{base}}\nX-Ids: {{ids}}"} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			_, _ = io.WriteString(w, `[9223372036854775808,9223372036854775809]`)
		}))
		code, out, stderr := run(t, "test", scenario(t, "GET {{base}}\nHTTP 200\n[Captures]\nids: jsonpath \"$[*]\"\n"+next), "--var", "base="+server.URL, "--secret", "ids")
		server.Close()
		if code != 2 || hits.Load() != 1 || strings.Contains(out+stderr, "922337203685477580") {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
	}
}
