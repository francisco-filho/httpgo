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

// GO-410: the published source is executable, the opt-in is explicit in help,
// and unsupported structural forms never generate partial execution.
func TestDynamicFilterPublishedExample(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/dynamic_filter.httpgo")
	if err != nil {
		t.Fatal(err)
	}
	guide, err := os.ReadFile("../../docs/captures.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(guide), strings.TrimSpace(string(input))) {
		t.Fatal("capture guide differs from versioned scenario")
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/first":
			_, _ = io.WriteString(w, `{"number":42}`)
		case "/orders":
			_, _ = io.WriteString(w, `{"items":[{"number":41,"status":"PLACED","id":"wrong"},{"number":42,"status":"PLACED","id":"selected"}]}`)
		case "/selected":
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", "../../testdata/scenarios/dynamic_filter.httpgo", "--render-asserts", "--var", "base_url="+server.URL)
	if code != 0 || stderr != "" || hits.Load() != 3 {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%s", code, stderr)
		}
		for _, term := range []string{"Selecionar com valores anteriores — US-31", `jsonpath "$.items[?(@.number=={{order_number}})].status" == "PLACED"`, "operandos vinculados ao plano validado", "sem renderizar/reanalisar a expressão", "inclusive em literal", "antes de decode/seleção", "dynamic_filter.httpgo"} {
			if !strings.Contains(out, term) {
				t.Errorf("missing help: %s", term)
			}
		}
	}
}

func TestDynamicFilterStructuralCorpusByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, name := range []string{"property", "index", "operator", "path", "relative", "concat", "invalid_template"} {
		file := "../../testdata/scenarios/unsupported_dynamic_filter_" + name + ".hurl"
		category := "construção não suportada"
		if name == "invalid_template" {
			file = "../../testdata/scenarios/dynamic_filter_invalid_template.hurl"
			category = "sintaxe inválida"
		}
		code, out, stderr := run(t, "test", file, "--render-asserts", "--var", "base_url="+server.URL)
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, category) {
			t.Fatalf("%s: code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
}
