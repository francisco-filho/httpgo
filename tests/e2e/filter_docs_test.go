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

// GO-356: the published scenario is the same one exercised against real HTTP;
// documentation describes only the implemented grammar, including the explicit US-31 opt-in evolution.
func TestFilterHelpAndPublishedExample(t *testing.T) {
	assertDocumentContains(t, "docs/subset.md", `pizza_id: jsonpath "$[?(@.slug=='pizza')].id"`, `jsonpath "$[?(@.slug=='pizza')].active" == true`, "Seleção estática por igualdade", "atributo ausente não corresponde nem a `null`", "exigem `--render-asserts`", "literais dos filtros são sempre ocultados", "scenario_filter.httpgo")
	input, err := os.ReadFile("../../testdata/scenarios/scenario_filter.httpgo")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"../../docs/captures.md", "../../docs/subset.md"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), strings.TrimSpace(string(input))) {
			t.Errorf("%s differs from published example", file)
		}
	}
}

func TestFilterPublishedEscapes(t *testing.T) {
	data, err := os.ReadFile("../../docs/subset.md")
	if err != nil {
		t.Fatal(err)
	}
	marker := "Exemplos de consultas na sintaxe `.httpgo`:"
	_, tail, ok := strings.Cut(string(data), marker)
	if !ok {
		t.Fatal("missing escapes example")
	}
	_, tail, ok = strings.Cut(tail, "```hurl\n")
	if !ok {
		t.Fatal("missing code block")
	}
	source, _, ok := strings.Cut(tail, "\n```")
	if !ok {
		t.Fatal("unterminated code block")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"name":"it's pizza","id":1},{"name":"a\"b","id":2},{"name":"🍕","id":3}]`)
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", scenario(t, source), "--var", "base_url="+server.URL)
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
	}
}

func TestFilterStructuralCorpusByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, name := range []string{"invalid_filter_literal", "unsupported_filter_ordering", "unsupported_filter_logic", "unsupported_filter_dynamic", "unsupported_filter_composite", "unsupported_filter_function", "unsupported_filter_regex", "unsupported_capture_filter"} {
		code, out, stderr := run(t, "test", "../../testdata/scenarios/"+name+".hurl", "--var", "base_url="+server.URL)
		category := "construção não suportada"
		if strings.HasPrefix(name, "invalid_") {
			category = "sintaxe inválida"
		}
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, category) {
			t.Fatalf("%s: code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
}
