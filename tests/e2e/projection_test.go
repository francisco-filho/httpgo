package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProjectionByBinary(t *testing.T) {
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{"issues":[{"code":"REQUIRED"},{"code":"INVALID"}]}`, 0},
		{`{"issues":[{"code":"INVALID"},{"code":"REQUIRED"}]}`, 0},
		{`{"issues":[{"code":"INVALID"}]}`, 1},
		{`{"issues":[{"code":"REQUIRED"},{"code":"OTHER_TENANT"}]}`, 1},
		{`{"issues":[{"code":"REQUIRED"},null]}`, 2},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.Method == "POST" {
					data, _ := io.ReadAll(r.Body)
					if !strings.Contains(string(data), `"REQUIRED"`) || !strings.Contains(string(data), `"INVALID"`) {
						t.Errorf("wrong captured body: %s", data)
					}
					w.WriteHeader(204)
					return
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			source := "GET " + server.URL + "/validation\nHTTP 200\n[Asserts]\njsonpath \"$.issues[*].code\" contains \"REQUIRED\"\njsonpath \"$.issues[*].code\" not contains \"OTHER_TENANT\"\n[Captures]\ncodes: jsonpath \"$.issues[*].code\"\nPOST " + server.URL + "/echo\n{{codes}}\nHTTP 204"
			code, out, stderr := run(t, "test", scenario(t, source))
			want := int32(1)
			if tc.code == 0 {
				want = 2
			}
			if code != tc.code || hits.Load() != want || tc.code == 0 && stderr != "" || tc.code != 0 && !strings.Contains(stderr, ":4:1:") && !strings.Contains(stderr, ":5:1:") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}

func TestProjectionInvalidLateNeverSends(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, path := range []string{"$[*", "$[**]", "$.*", "$..x", "$[0:1]", "$[0,1]", "$[?(@.x)]", "$[?(@.x == 1)]", "$.length()", "$.{{field}}", "$[{{index}}]"} {
		for _, nl := range []string{"\n", "\r\n"} {
			for _, declaration := range []string{"[Asserts]\njsonpath \"" + path + "\" exists", "[Captures]\nx: jsonpath \"" + path + "\""} {
				code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll("GET "+server.URL+"/first\nGET "+server.URL+"/late\nHTTP *\n"+declaration, "\n", nl)))
				column := ":5:11:"
				if strings.HasPrefix(declaration, "[Captures]") {
					column = ":5:14:"
				}
				if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, column) {
					t.Fatalf("%s: code=%d hits=%d out=%s err=%s", path, code, hits.Load(), out, stderr)
				}
			}
		}
	}
}
