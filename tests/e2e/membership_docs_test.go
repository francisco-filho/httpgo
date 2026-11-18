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

func TestMembershipHelpAndPublishedExample(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
		}
		for _, term := range []string{`jsonpath "$.codes" contains "REQUIRED"`, `jsonpath "$.codes" not contains "OTHER_TENANT"`, `jsonpath "$.passwordHash" not exists`, `header "Set-Cookie" not contains "Domain"`, "Tipo presente incompatível falha com 1", "Sem negação genérica", "nunca arrays/objetos selecionados"} {
			if !strings.Contains(out, term) {
				t.Errorf("help missing %q", term)
			}
		}
	}
	input, err := os.ReadFile("../../testdata/scenarios/scenario_assert_membership.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"../../docs/assertions.md", "../../docs/subset.md"} {
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
		if r.URL.Path != "/validation" {
			t.Error("unexpected path")
		}
		w.Header().Set("Set-Cookie", "sid=fictitious; Secure")
		_, _ = io.WriteString(w, `{"codes":["REQUIRED","INVALID"],"name":"Pizza"}`)
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", "../../testdata/scenarios/scenario_assert_membership.hurl", "--var", "base_url="+server.URL)
	if code != 0 || stderr != "" || hits.Load() != 1 {
		t.Fatalf("code=%d out=%s err=%s hits=%d", code, out, stderr, hits.Load())
	}
}

func TestMembershipStructuralCorpusByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, name := range []string{"invalid_assert_membership_scalar", "invalid_assert_not_exists_operand", "invalid_assert_header_not_contains", "unsupported_assert_json_negation", "unsupported_assert_header_not_exists", "unsupported_assert_count_contains", "unsupported_assert_membership_wildcard"} {
		code, out, stderr := run(t, "test", "../../testdata/scenarios/"+name+".hurl", "--var", "base_url="+server.URL)
		category := "sintaxe inválida"
		if strings.HasPrefix(name, "unsupported_") {
			category = "construção não suportada"
		}
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, category) || !strings.Contains(stderr, ":5:") {
			t.Fatalf("%s code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
}
