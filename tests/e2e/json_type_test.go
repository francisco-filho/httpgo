package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-361–366, GO-370–374: real HTTP and binary exit codes, not just parsing.
func TestJSONTypeByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, expression, body, diagnostic string
		code                               int
	}{
		{"demo", "jsonpath \"$.token\" isString\njsonpath \"$.attempts\" isInteger\njsonpath \"$.active\" isBoolean\njsonpath \"$.issues\" isEmpty", `{"token":"abc","attempts":3,"active":false,"issues":[]}`, "", 0},
		{"large integer", `jsonpath "$" isInteger`, `999999999999999999999999999999999999`, "", 0},
		{"decimal", `jsonpath "$" isInteger`, `3.0`, "notação não inteira", 1},
		{"exponent", `jsonpath "$" isInteger`, `3e0`, "notação não inteira", 1},
		{"numeric string", `jsonpath "$" isInteger`, `"3"`, "recebido tipo string", 1},
		{"empty string", `jsonpath "$" isString`, `""`, "", 0},
		{"true", `jsonpath "$" isBoolean`, `true`, "", 0},
		{"false", `jsonpath "$" isBoolean`, `false`, "", 0},
		{"string type mismatch", `jsonpath "$" isString`, `{"body-private":[]}`, "recebido tipo objeto", 1},
		{"boolean type mismatch", `jsonpath "$" isBoolean`, `0`, "recebido tipo número", 1},
		{"empty text", `jsonpath "$" isEmpty`, `""`, "", 0},
		{"empty array", `jsonpath "$" isEmpty`, `[]`, "", 0},
		{"empty object", `jsonpath "$" isEmpty`, `{}`, "", 0},
		{"whitespace", `jsonpath "$" isEmpty`, `" "`, "recebido tipo string", 1},
		{"null element", `jsonpath "$" isEmpty`, `[null]`, "recebido tipo array", 1},
		{"null property", `jsonpath "$" isEmpty`, `{"x":null}`, "recebido tipo objeto", 1},
		{"null", `jsonpath "$" isEmpty`, `null`, "recebido tipo null", 1},
		{"missing", `jsonpath "$.missing" isEmpty`, `{}`, "recebido ausente", 1},
		{"missing index", `jsonpath "$[1]" isInteger`, `[]`, "recebido ausente", 1},
		{"traversal", `jsonpath "$[0]" isEmpty`, `{}`, "erro de avaliação JSON", 2},
		{"empty response", `jsonpath "$" isEmpty`, ``, "erro de avaliação JSON", 2},
		{"malformed", `jsonpath "$" isEmpty`, `{`, "erro de avaliação JSON", 2},
		{"additional", `jsonpath "$" isEmpty`, `[] []`, "erro de avaliação JSON", 2},
		{"numeric equality", "jsonpath \"$.a\" == 3e0\njsonpath \"$.b\" == 3\njsonpath \"$.c\" == 3.0\njsonpath \"$.items\" count == 1e0\njsonpath \"$.a\" >= 3\njsonpath \"$.s\" matches \"^abc$\"", `{"a":3,"b":3.0,"c":3e0,"items":[null],"s":"abc"}`, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); _, _ = io.WriteString(w, tc.body) }))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/first\nHTTP *\n[Asserts]\n  "+tc.expression+"\nGET "+server.URL+"/next"))
			wantHits := int32(1)
			if tc.code == 0 {
				wantHits = 2
			}
			if code != tc.code || hits.Load() != wantHits || !strings.Contains(stderr, tc.diagnostic) || strings.Contains(out+stderr, "body-private") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			if tc.code != 0 && (!strings.Contains(stderr, ":4:3:") || !strings.Contains(out, "não_iniciadas=1")) {
				t.Fatalf("out=%s err=%s", out, stderr)
			}
			if strings.Contains(stderr, "esperado null") {
				t.Fatalf("unary predicate presented with operand: %s", stderr)
			}
		})
	}
}

func TestJSONTypeInvalidNeverSends(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, newline := range []string{"\n", "\r\n"} {
		for _, line := range []string{`jsonpath "$" isInteger 3`, `jsonpath "$" isString ""`, `jsonpath "$" isBoolean false`, `jsonpath "$" isEmpty null`, `jsonpath "$" isNumber`, `jsonpath "$" not isInteger`, `jsonpath "$" count isInteger`, `jsonpath "$" count isString`, `jsonpath "$" count isBoolean`, `jsonpath "$" count isEmpty`, `jsonpath "$." isEmpty`} {
			text := "GET " + server.URL + "/first\nGET " + server.URL + "/late\nHTTP *\n[Asserts]\n  " + line
			code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll(text, "\n", newline)))
			if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":5:") {
				t.Fatalf("%s: code=%d hits=%d out=%s err=%s", line, code, hits.Load(), out, stderr)
			}
		}
	}
}
