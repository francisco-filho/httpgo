package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-372: the documented contract and the verbatim published example are executable.
func TestJSONTypeHelpAndPublishedExample(t *testing.T) {
	assertDocumentContains(t, "docs/subset.md", `jsonpath "$.token" isString`, `jsonpath "$.attempts" isInteger`, `jsonpath "$.active" isBoolean`, `jsonpath "$.issues" isEmpty`, "notação inteira", "sem limite de inteiro de máquina", "sem imprimir valores ou estruturas", "sem esperado", "Não há schema", "outros tipos ou negação genérica")
	input, err := os.ReadFile("../../testdata/scenarios/json_types.httpgo")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"../../docs/assertions.md", "../../docs/subset.md"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), strings.TrimSpace(string(input))) {
			t.Errorf("%s differs from published example", file)
		}
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/session-info" {
			t.Errorf("path=%s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"token":"abc","attempts":3,"active":false,"issues":[]}`)
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", "../../testdata/scenarios/json_types.httpgo", "--var", "base_url="+server.URL)
	if code != 0 || stderr != "" || hits.Load() != 1 {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}

func TestJSONTypeStructuralCorpusByBinary(t *testing.T) {
	for _, name := range []string{"invalid_json_type_operand", "unsupported_json_type_count", "unsupported_json_type_negation", "unsupported_json_type_other"} {
		code, out, stderr := run(t, "test", "../../testdata/scenarios/"+name+".hurl")
		category := "construção não suportada"
		if strings.HasPrefix(name, "invalid_") {
			category = "sintaxe inválida"
		}
		if code != 2 || !prevalidationSummary(out) || !strings.Contains(stderr, category) {
			t.Fatalf("%s: code=%d out=%s err=%s", name, code, out, stderr)
		}
	}
}
