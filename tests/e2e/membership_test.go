package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMembershipDemonstrationByBinary(t *testing.T) {
	const lines = "jsonpath \"$.codes\" contains \"REQUIRED\"\njsonpath \"$.codes\" not contains \"OTHER_TENANT\"\njsonpath \"$.passwordHash\" not exists\nheader \"Set-Cookie\" not contains \"Domain\""
	for _, tc := range []struct {
		name, body, cookie string
		code               int
	}{
		{"demo", `{"codes":["REQUIRED","INVALID"],"name":"Pizza"}`, "sid=private; Secure", 0},
		{"required removed", `{"codes":["INVALID"]}`, "sid=private", 1},
		{"other tenant", `{"codes":["REQUIRED","OTHER_TENANT"]}`, "sid=private", 1},
		{"null present", `{"codes":["REQUIRED"],"passwordHash":null}`, "sid=private", 1},
		{"domain", `{"codes":["REQUIRED"]}`, "sid=private; Domain=x", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/Pizza" {
					w.WriteHeader(204)
					return
				}
				w.Header().Add("Set-Cookie", tc.cookie)
				w.Header().Add("Set-Cookie", "Domain=ignored")
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/validation\nHTTP 200\n[Asserts]\n"+lines+"\n[Captures]\nname: jsonpath \"$.name\"\nGET "+server.URL+"/{{name}}\nHTTP 204"))
			wantHits := int32(1)
			if tc.code == 0 {
				wantHits = 2
			}
			if code != tc.code || hits.Load() != wantHits || strings.Contains(out+stderr, "private") || strings.Contains(stderr, "erro de captura") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			if tc.code == 0 && stderr != "" || tc.code == 1 && (!strings.Contains(stderr, "assertion") || !strings.Contains(out, "não_iniciadas=1")) {
				t.Fatalf("out=%s err=%s", out, stderr)
			}
		})
	}
}

func TestMembershipByBinary(t *testing.T) {
	for _, tc := range []struct {
		line, body string
		code       int
	}{
		{`jsonpath "$" contains true`, `[true,null,"true"]`, 0},
		{`jsonpath "$" contains null`, `[null]`, 0},
		{`jsonpath "$" contains 1`, `["1"]`, 1},
		{`jsonpath "$" contains 9007199254740993`, `[9007199254740992]`, 1},
		{`jsonpath "$" contains 1e99999999999999999999`, `[10e99999999999999999998]`, 0},
		{`jsonpath "$" not contains 1`, `[[1],{"x":1}]`, 0},
		{`jsonpath "$" not contains ""`, `[]`, 0},
		{`jsonpath "$" not contains ""`, `""`, 1},
		{`jsonpath "$" contains "Pizza"`, `"Mini Pizza"`, 0},
		{`jsonpath "$" not contains null`, `null`, 1},
		{`jsonpath "$.missing" not contains null`, `{}`, 0},
		{`jsonpath "$[1]" not exists`, `[null]`, 0},
		{`jsonpath "$" not exists`, `false`, 1},
		{`jsonpath "$" not contains 1`, `{}`, 1},
		{`jsonpath "$.x.y" not exists`, `{"x":null}`, 2},
		{`jsonpath "$" not contains 1`, `[] []`, 2},
		{`jsonpath "$" not exists`, ``, 2},
	} {
		t.Run(tc.line+tc.body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, tc.body) }))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"\nHTTP *\n[Asserts]\n  "+tc.line))
			if code != tc.code || tc.code == 0 && stderr != "" || tc.code != 0 && !strings.Contains(stderr, ":4:3:") {
				t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
			}
		})
	}
}

func TestMembershipInvalidNeverSends(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, line := range []string{`jsonpath "$" contains []`, `jsonpath "$" not contains {}`, `jsonpath "$" not exists null`, `header "X" not contains 1`, `jsonpath "$" not`, `jsonpath "$" count not contains 1`, `jsonpath "$" not matches "x"`, `header "X" not exists`} {
		code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/first\nGET "+server.URL+"/late\nHTTP *\n[Asserts]\n"+line))
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":5:1:") {
			t.Fatalf("%s code=%d hits=%d out=%s err=%s", line, code, hits.Load(), out, stderr)
		}
	}
}
