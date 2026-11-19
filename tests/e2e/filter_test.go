package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-344: real binary, real HTTP, and the request proves the captured ID.
func TestFilterByBinary(t *testing.T) {
	for _, tc := range []struct {
		body string
		code int
	}{
		{`[{"slug":"pizza","id":"p1","active":true},{"slug":"juice","id":"p2","active":false}]`, 0},
		{`[{"slug":"juice","id":"p2","active":false},{"slug":"pizza","id":"p1","active":true}]`, 0},
		{`[{"slug":"pizza","id":"p1","active":false},{"slug":"juice","id":"p2","active":true}]`, 1},
		{`[{"slug":"pizza","id":"p1","active":true},null]`, 2},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/products/p1" {
					w.WriteHeader(200)
					return
				}
				if r.URL.Path != "/products" {
					t.Errorf("wrong entity: %s", r.URL.Path)
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", filepath.Join("..", "..", "testdata", "scenarios", "scenario_filter.hurl"), "--var", "base_url="+server.URL)
			want := int32(1)
			if tc.code == 0 {
				want = 2
			}
			if code != tc.code || hits.Load() != want || tc.code == 0 && stderr != "" || strings.Contains(out+stderr, `"juice"`) {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}

// GO-352: even a late invalid query prevents every send, assertions and captures.
func TestFilterInvalidLateNeverSends(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, path := range []string{`$[?(@.x != 1)]`, `$[?(@.x == 'bad)]`, `$[?(@.x == {{value}})]`, `$[?(@.x == [])]`, `$[?(@.x == 1 && @.y == 2)]`, `$[?(@.x[*] == 1)]`} {
		for _, nl := range []string{"\n", "\r\n"} {
			for _, capture := range []bool{false, true} {
				declaration := "[Asserts]\njsonpath " + strconv.Quote(path) + " exists"
				column := ":5:11:"
				if capture {
					declaration = "[Captures]\nx: jsonpath " + strconv.Quote(path)
					column = ":5:14:"
				}
				code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll("GET "+server.URL+"/first\nGET "+server.URL+"/late\nHTTP *\n"+declaration, "\n", nl)))
				if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, column) {
					t.Fatalf("%s code=%d hits=%d out=%s err=%s", path, code, hits.Load(), out, stderr)
				}
			}
		}
	}
}
