package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHeaderMatchesHelpAndPublishedExample(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
		}
		for _, term := range []string{`header "Set-Cookie" matches "^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$"`, "Somente primeira ocorrência", "Padrão string JSON literal", "matches vazio", "Sem regex de body/variável", "sem interpolação"} {
			if !strings.Contains(out, term) {
				t.Errorf("help missing %q", term)
			}
		}
	}
	input, err := os.ReadFile("../../testdata/scenarios/scenario_assert_header_matches.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"../../README.md", "../../docs/subset.md"} {
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
		if r.URL.Path != "/session" {
			t.Error("unexpected path")
		}
		w.Header().Set("Set-Cookie", "session=fictitious; Path=/; HttpOnly; Secure; SameSite=Lax")
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", "../../testdata/scenarios/scenario_assert_header_matches.hurl", "--var", "base_url="+server.URL)
	if code != 0 || stderr != "" || hits.Load() != 1 {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}

func TestHeaderMatchesStructuralCorpusByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, name := range []string{"invalid_assert_header_matches_pattern", "invalid_assert_header_matches_escape", "invalid_assert_header_matches_unquoted", "invalid_assert_header_matches_extra", "unsupported_assert_header_matches_lookaround", "unsupported_assert_header_matches_backreference"} {
		code, out, stderr := run(t, "test", "../../testdata/scenarios/"+name+".hurl", "--var", "base_url="+server.URL)
		category := "sintaxe inválida"
		if strings.HasPrefix(name, "unsupported_") {
			category = "construção não suportada"
		}
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, category) || !strings.Contains(stderr, ":5:1:") || strings.Contains(stderr, "private") {
			t.Fatalf("%s code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
}

func TestHeaderMatchesSemanticsByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, pattern string
		values        []string
		code          int
	}{
		{"partial", `"Pizza"`, []string{"Mini Pizza"}, 0},
		{"anchored", `"^Pizza$"`, []string{"Mini Pizza"}, 1},
		{"case", `"pizza"`, []string{"Pizza"}, 1},
		{"option", `"(?i)^pizza$"`, []string{"Pizza"}, 0},
		{"Unicode", `"^\\p{L}+$"`, []string{"雪é"}, 0},
		{"normalization", `"^é$"`, []string{"é"}, 1},
		{"escapes", `"^a\\s\"b\\\\c/$"`, []string{"a \"b\\c/"}, 0},
		{"empty pattern value", `""`, []string{"x"}, 0},
		{"empty pattern empty", `""`, []string{""}, 0},
		{"absent", `""`, nil, 1},
		{"empty anchor", `"^$"`, []string{""}, 0},
		{"empty anchor value", `"^$"`, []string{"x"}, 1},
		{"first", `"^ok$"`, []string{"ok", "bad"}, 0},
		{"second", `"^ok$"`, []string{"bad", "ok"}, 1},
		{"first empty", `"^$"`, []string{"", "bad"}, 0},
		{"second after empty", `"^ok$"`, []string{"", "ok"}, 1},
		{"comma", `"^first,whole$"`, []string{"first,whole", "other"}, 0},
		{"literal", `"^{{name}}$"`, []string{"{{name}}"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				for _, v := range tc.values {
					w.Header().Add("X-Test", v)
				}
			}))
			defer server.Close()
			for _, extra := range [][]string{nil, {"--var", "name=resolved"}} {
				args := []string{"test", scenario(t, "GET {{base}}/first\nHTTP *\n[Asserts]\nheader \"x-TEST\" matches "+tc.pattern+"\nGET {{base}}/next"), "--var", "base=" + server.URL}
				args = append(args, extra...)
				hits.Store(0)
				code, out, stderr := run(t, args...)
				wantHits := int32(2)
				if tc.code != 0 {
					wantHits = 1
				}
				if code != tc.code || hits.Load() != wantHits {
					t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
				}
			}
		})
	}
}
