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

func TestProjectionHelpAndPublishedExample(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%s", code, stderr)
		}
		for _, term := range []string{`jsonpath "$.issues[*].code" contains "REQUIRED"`, "zero resultados = ausência", "count conta o array reduzido", "Sem .*, slices, unions", "sem coleção ou esperado composto"} {
			if !strings.Contains(out, term) {
				t.Errorf("help missing %q", term)
			}
		}
	}
	input, err := os.ReadFile("../../testdata/scenarios/scenario_projection.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"../../README.md", "../../docs/subset.md"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), strings.TrimSpace(string(input))) {
			t.Errorf("%s differs from published scenario", file)
		}
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/validation" {
			_, _ = io.WriteString(w, `{"issues":[{"code":"INVALID"},{"code":"REQUIRED"}]}`)
			return
		}
		if r.URL.Path != "/echo" {
			t.Error("unexpected path")
		}
		data, _ := io.ReadAll(r.Body)
		if string(data) != `["INVALID","REQUIRED"]` {
			t.Errorf("wrong projection: %s", data)
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", "../../testdata/scenarios/scenario_projection.hurl", "--var", "base_url="+server.URL)
	if code != 0 || stderr != "" || hits.Load() != 2 {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}

func TestProjectionStructuralCorpusByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, name := range []string{"invalid_projection_wildcard", "unsupported_projection_slice", "unsupported_projection_union", "unsupported_projection_recursive", "unsupported_projection_filter"} {
		code, out, stderr := run(t, "test", "../../testdata/scenarios/"+name+".hurl", "--var", "base_url="+server.URL)
		category := "construção não suportada"
		if strings.HasPrefix(name, "invalid_") {
			category = "sintaxe inválida"
		}
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, category) || !strings.Contains(stderr, ":5:") {
			t.Fatalf("%s code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
}

func TestProjectionReducedTypesByBinary(t *testing.T) {
	for _, tc := range []struct {
		body, line string
		code       int
	}{
		{`[1,2]`, `jsonpath "$[*]" count == 2`, 0},
		{`[1]`, `jsonpath "$[*]" count == 1`, 1},
		{`[]`, `jsonpath "$[*]" count == 0`, 1},
		{`[]`, `jsonpath "$[*]" not exists`, 0},
		{`[null]`, `jsonpath "$[*]" == null`, 0},
		{`[[]]`, `jsonpath "$[*]" exists`, 0},
		{`[[]]`, `jsonpath "$[*]" count == 0`, 0},
		{`[[],[1]]`, `jsonpath "$[*]" == [[],[1]]`, 0},
		{`[{},{"x":null}]`, `jsonpath "$[*].x" == null`, 0},
		{`[[[1,2]]]`, `jsonpath "$[*][0][*]" == [1,2]`, 0},
		{`[{"x":1},null]`, `jsonpath "$[*].x" not exists`, 2},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, tc.body) }))
		code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"\nHTTP *\n[Asserts]\n"+tc.line))
		server.Close()
		if code != tc.code || tc.code == 0 && stderr != "" || tc.code != 0 && !strings.Contains(stderr, ":4:1:") {
			t.Fatalf("%s %s: code=%d out=%s err=%s", tc.body, tc.line, code, out, stderr)
		}
	}
}
