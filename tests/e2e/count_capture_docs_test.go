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

func TestCountCaptureDocumentation(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/scenario_count_unchanged.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../README.md", "../../docs/subset.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), strings.TrimSpace(string(input))) {
			t.Fatalf("example differs: %s", path)
		}
	}
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%s", code, stderr)
		}
		for _, term := range []string{"Captura de contagem — US-33", `orders_before: jsonpath "$.items" count`, "número JSON inteiro não negativo", "sem baseline zero fictício", "esperado dinâmico exige", "Sem parâmetros, pipelines, aritmética", "--secret orders_before", "scenario_count_unchanged.httpgo"} {
			if !strings.Contains(out, term) {
				t.Errorf("help missing %q", term)
			}
		}
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/orders" {
			_, _ = io.WriteString(w, `{"items":[null,null,null]}`)
		}
		if r.URL.Path == "/counts" {
			body, err := io.ReadAll(r.Body)
			if err != nil || string(body) != `{"count":3}` {
				t.Errorf("body=%s err=%v", body, err)
			}
		}
	}))
	defer server.Close()
	for _, name := range []string{"unsupported_capture_count_pipeline.hurl", "unsupported_capture_header_count.hurl", "unsupported_capture_count_parameters.hurl", "unsupported_capture_count_arithmetic.hurl", "invalid_capture_count_separator.hurl"} {
		code, out, stderr := run(t, "test", "../../testdata/scenarios/"+name)
		category := "construção não suportada"
		if strings.HasPrefix(name, "invalid_") {
			category = "sintaxe inválida"
		}
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, category) || !strings.Contains(stderr, ":7:") {
			t.Fatalf("%s: code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
	for _, name := range []string{"scenario_count_unchanged.httpgo", "capture_count.httpgo"} {
		code, out, stderr := run(t, "test", "../../testdata/scenarios/"+name, "--render-asserts", "--var", "base_url="+server.URL)
		if code != 0 {
			t.Fatalf("%s: code=%d out=%s err=%s", name, code, out, stderr)
		}
	}
	if hits.Load() != 5 {
		t.Fatalf("hits=%d", hits.Load())
	}
}
