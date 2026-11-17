package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-426: help and published examples describe only proven value inequality.
func TestJSONInequalityDocumentation(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/scenario_json_inequality_rotation.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../README.md", "../../docs/subset.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), strings.TrimSpace(string(input))) {
			t.Fatalf("published example differs: %s", path)
		}
	}
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%s", code, stderr)
		}
		for _, term := range []string{"Desigualdade JSON — US-32", `jsonpath "$.accessToken" != "{{old_token}}"`, "--secret old_token --secret new_token", "repetição ou ausência falha com 1", "count !=, header/body !=", "não garante revogação", "validade do novo", "scenario_json_inequality_rotation.hurl"} {
			if !strings.Contains(out, term) {
				t.Errorf("help missing %q", term)
			}
		}
	}
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
