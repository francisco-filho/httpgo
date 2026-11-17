package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-406–408,411: real HTTP confirms typed collections, no partial advancement
// and strict errors, with no assertion-per-item or first-ID shortcuts.
func TestDynamicFilterCaptureContracts(t *testing.T) {
	for _, tc := range []struct {
		name, body, asserts, captures, next string
		code, hits                          int
	}{
		{"plural", `[{"key":1,"id":[1,2]},{"key":1,"id":[]}]`, `jsonpath "$[?(@.key=={{key}})].id" count == 2`, `ids: jsonpath "$[?(@.key=={{key}})].id"`, "POST {{base}}/echo\n{{ids}}", 0, 2},
		{"null", `[{"key":1,"id":null}]`, `jsonpath "$[?(@.key=={{key}})].id" == null`, `ids: jsonpath "$[?(@.key=={{key}})].id"`, "POST {{base}}/echo\n{{ids}}", 0, 2},
		{"plural URL", `[{"key":1,"id":"a"},{"key":1,"id":"b"}]`, `jsonpath "$[?(@.key=={{key}})].id" count == 2`, `ids: jsonpath "$[?(@.key=={{key}})].id"`, "GET {{base}}/{{ids}}", 2, 1},
		{"same batch", `[{"key":1,"id":1}]`, `jsonpath "$[?(@.key=={{key}})].id" == 1`, "ids: jsonpath \"$[?(@.key=={{key}})].id\"\nlate: jsonpath \"$[?(@.id=={{ids}})]\"", "GET {{base}}/never", 2, 1},
		{"strict branch", `[{"key":1,"id":1},null]`, `jsonpath "$[?(@.key=={{key}})].id" not exists`, `ids: jsonpath "$"`, "GET {{base}}/never", 2, 1},
		{"capture empty", `[]`, `jsonpath "$[?(@.key=={{key}})].id" not exists`, `ids: jsonpath "$[?(@.key=={{missing}})].id"`, "GET {{base}}/never", 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/echo" {
					body, _ := io.ReadAll(r.Body)
					want := "[[1,2],[]]"
					if tc.name == "null" {
						want = "null"
					}
					if string(body) != want {
						t.Errorf("body=%s want=%s", body, want)
					}
				} else if r.URL.Path != "/items" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			source := "GET {{base}}/items\nHTTP 200\n[Asserts]\n" + tc.asserts + "\n[Captures]\n" + tc.captures + "\n" + tc.next
			code, out, stderr := run(t, "test", scenario(t, source), "--render-asserts", "--var", "base="+server.URL, "--var", "key=1")
			if code != tc.code || int(hits.Load()) != tc.hits {
				t.Fatalf("code=%d hits=%d stdout=%s stderr=%s", code, hits.Load(), out, stderr)
			}
			if tc.code == 2 && strings.Contains(out, tc.body) {
				t.Fatalf("collection leaked: %s", out)
			}
		})
	}
}
