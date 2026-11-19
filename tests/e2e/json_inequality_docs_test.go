package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-426: documentation and published examples describe only proven value inequality.
func TestJSONInequalityDocumentation(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/scenario_json_inequality_rotation.httpgo")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../docs/assertions.md", "../../docs/subset.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), strings.TrimSpace(string(input))) {
			t.Fatalf("published example differs: %s", path)
		}
	}
	assertDocumentContains(t, "docs/subset.md", "Desigualdade JSON — US-32", `jsonpath "$.accessToken" != "{{old_token}}"`, "--secret old_token --secret new_token", "Ausência falha com", "count !=", "desigualdade de header/body", "não garante revogação", "validade do novo", "scenario_json_inequality_rotation.httpgo")
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, name := range []string{"header", "body", "negation"} {
		code, out, stderr := run(t, "test", "../../testdata/scenarios/unsupported_inequality_"+name+".hurl", "--var", "base_url="+server.URL)
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":5:") || !strings.Contains(stderr, "construção não suportada") {
			t.Fatalf("%s: code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
}
