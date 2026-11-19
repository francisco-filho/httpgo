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

func TestRenderAssertsHelpAndPublishedExample(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
		}
		for _, term := range []string{"--render-asserts", "evolução explícita HTTPGo", "sem valor", "antes dos operandos da consulta", "sem escape automático", "nunca é recursiva", "Captura própria válida fica disponível", `jsonpath "$.number" == {{order_number}}`, `jsonpath "$.name" == "{{order_name}}"`} {
			if !strings.Contains(out, term) {
				t.Errorf("help missing %q", term)
			}
		}
	}
	input, err := os.ReadFile("../../testdata/scenarios/scenario_render_asserts.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../docs/assertions.md", "../../docs/subset.md"} {
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(text), strings.TrimSpace(string(input))) {
			t.Errorf("published example differs in %s", path)
		}
	}
	for _, different := range []bool{false, true} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if different && r.Method == "GET" {
				_, _ = io.WriteString(w, `{"number":9007199254740992,"name":"Pizza"}`)
				return
			}
			_, _ = io.WriteString(w, `{"number":9007199254740993,"name":"Pizza"}`)
		}))
		code, out, stderr := run(t, "test", "../../testdata/scenarios/scenario_render_asserts.hurl", "--render-asserts", "--var", "base_url="+server.URL)
		server.Close()
		want := 0
		if different {
			want = 1
		}
		if code != want || hits.Load() != 2 {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
	}
}

func TestRenderAssertsStructuralCorpusByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, name := range []string{"invalid_render_asserts_template", "invalid_render_asserts_extra", "unsupported_render_asserts_body_regex", "unsupported_render_asserts_query"} {
		code, out, stderr := run(t, "test", "../../testdata/scenarios/"+name+".hurl", "--render-asserts", "--var", "base_url="+server.URL)
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":6:") {
			t.Fatalf("%s code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
}
