package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestJSONStructureByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, expression, body, diagnostic string
		code                               int
	}{
		{"null exists", `jsonpath "$.id" exists`, `{"id":null}`, "", 0},
		{"empty exists", "jsonpath \"$.s\" exists\njsonpath \"$.a\" exists\njsonpath \"$.o\" exists\njsonpath \"$.b\" exists\njsonpath \"$.n\" exists", `{"s":"","a":[],"o":{},"b":false,"n":0}`, "", 0},
		{"root exists", `jsonpath "$" exists`, `null`, "", 0},
		{"missing", `jsonpath "$.id" exists`, `{}`, "exists: esperado presente, recebido ausente", 1},
		{"index missing", `jsonpath "$[0]" exists`, `[]`, "recebido ausente", 1},
		{"empty count", `jsonpath "$" count == 0`, `[]`, "", 0},
		{"null counted", `jsonpath "$" count == 3`, `[null,0,false]`, "", 0},
		{"count mismatch", `jsonpath "$" count == 2`, `["body-private"]`, "count ==: esperado 2, recebido 1", 1},
		{"strict upper", `jsonpath "$" count > 0`, `[]`, "recebido 0", 1},
		{"inclusive upper", `jsonpath "$" count >= 0`, `[]`, "", 0},
		{"strict lower", `jsonpath "$" count < 0`, `[]`, "recebido 0", 1},
		{"inclusive lower", `jsonpath "$" count <= 0`, `[]`, "", 0},
		{"large expected", `jsonpath "$" count < 999999999999999999999999999`, `[]`, "", 0},
		{"missing count", `jsonpath "$.id" count == 0`, `{}`, "recebido ausente", 1},
		{"string count", `jsonpath "$" count == 0`, `"body-private"`, "count exige array; recebido string", 1},
		{"object count", `jsonpath "$" count == 0`, `{"body-private":[]}`, "count exige array; recebido objeto", 1},
		{"null count", `jsonpath "$" count == 0`, `null`, "count exige array; recebido null", 1},
		{"number count", `jsonpath "$" count == 0`, `3`, "count exige array; recebido número", 1},
		{"bool count", `jsonpath "$" count == 0`, `true`, "count exige array; recebido booleano", 1},
		{"traversal", `jsonpath "$.id.x" exists`, `{"id":null}`, "erro de avaliação JSON", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/first\nHTTP *\n[Asserts]\n  "+tc.expression+"\nGET "+server.URL+"/next"))
			wantHits := int32(1)
			if tc.code == 0 {
				wantHits = 2
			}
			if code != tc.code || hits.Load() != wantHits || !strings.Contains(stderr, tc.diagnostic) || strings.Contains(out+stderr, "body-private") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			if tc.code == 0 && stderr != "" || tc.code != 0 && (!strings.Contains(stderr, ":4:3:") || !strings.Contains(out, "não_iniciadas=1")) {
				t.Fatalf("out=%s err=%s", out, stderr)
			}
		})
	}
}

func TestJSONStructureInvalidNeverSends(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, line := range []string{`jsonpath "$" exists null`, `jsonpath "$" count`, `jsonpath "$" count == -1`, `jsonpath "$" count == 1.1`, `jsonpath "$" count == "3"`, `jsonpath "$" count == true`, `jsonpath "$" count == 1 2`, `jsonpath "$" count exists`, `jsonpath "$" count != 1`, `jsonpath "$" count count == 1`} {
		code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/first\nGET "+server.URL+"/late\nHTTP *\n[Asserts]\n"+line))
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":5:1:") {
			t.Fatalf("%s: code=%d hits=%d out=%s err=%s", line, code, hits.Load(), out, stderr)
		}
	}
}
