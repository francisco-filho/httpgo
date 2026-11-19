package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRenderAssertsTextByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, line, value string
		code, hits        int
	}{
		{"body", `body contains "{{saved}}"`, `"a123"`, 0, 3},
		{"header", `header "X" contains "{{saved}}"`, `"a123"`, 0, 3},
		{"header regex", `header "X" matches "^{{saved}}$"`, `"a[0-9]+"`, 0, 3},
		{"JSON regex", `jsonpath "$.name" matches "^{{saved}}$"`, `"a[0-9]+"`, 0, 3},
		{"membership", `jsonpath "$.numbers" contains {{saved}}`, `9007199254740993`, 0, 3},
		{"wrong regex", `header "X" matches "^{{saved}}$"`, `"b[0-9]+"`, 1, 2},
		{"invalid regex", `header "X" matches "{{saved}}"`, `"["`, 2, 2},
		{"JSON invalid regex", `jsonpath "$.name" matches "{{saved}}"`, `"(?=x)"`, 2, 2},
		{"structure", `body not contains "{{saved}}"`, `[]`, 2, 2},
		{"missing", `header "Missing" not contains "{{missing}}"`, `null`, 2, 2},
		{"static regex", `header "X" matches "["`, `null`, 2, 0},
		{"body regex unsupported", `body matches "{{saved}}"`, `null`, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/capture" {
					_, _ = io.WriteString(w, tc.value)
					return
				}
				w.Header().Add("X", "a123")
				w.Header().Add("X", "second")
				_, _ = io.WriteString(w, `{"name":"a123","numbers":[9007199254740993]}`)
			}))
			defer server.Close()
			source := "GET {{base}}/capture\nHTTP 200\n[Captures]\nsaved: jsonpath \"$\"\nGET {{base}}/assert\nHTTP 200\n[Asserts]\n" + tc.line + "\nGET {{base}}/next"
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL, "--render-asserts")
			if code != tc.code || int(hits.Load()) != tc.hits {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			if tc.code == 2 && tc.hits != 0 && (!strings.Contains(stderr, "avaliação") || !strings.Contains(stderr, ":8:1:")) {
				t.Fatalf("wrong diagnostic: %s", stderr)
			}
		})
	}
}

func TestRenderAssertsLiteralProfileByBinary(t *testing.T) {
	for _, defined := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X", "{{missing}}")
			_, _ = io.WriteString(w, `{"value":"{{missing}}"}`)
		}))
		source := "GET {{base}}/\nHTTP 200\n[Asserts]\nheader \"X\" == \"{{missing}}\"\nheader \"X\" matches \"{{missing}}\"\nbody contains \"{{missing}}\"\njsonpath \"$.value\" == \"{{missing}}\"\njsonpath \"$.value\" matches \"{{missing}}\""
		args := []string{"test", scenario(t, source), "--var", "base=" + server.URL}
		if defined {
			args = append(args, "--var", "missing=resolved")
		}
		code, out, stderr := run(t, args...)
		server.Close()
		if code != 0 {
			t.Fatalf("defined=%v code=%d out=%s err=%s", defined, code, out, stderr)
		}
	}
}
