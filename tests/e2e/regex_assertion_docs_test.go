package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRegexAssertionStructuralCorpus(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, file := range []string{"invalid_regex_assertion_late.hurl", "invalid_regex_assertion_escape.hurl", "unsupported_regex_assertion_lookaround.hurl", "unsupported_regex_assertion_backreference.hurl", "scenario_unsupported_regex_assertion_pipeline.hurl"} {
		data, err := os.ReadFile("../../testdata/scenarios/" + file)
		if err != nil {
			t.Fatal(err)
		}
		for _, nl := range []string{"\n", "\r\n"} {
			input := strings.ReplaceAll(string(data), "http://example.test", server.URL)
			input = strings.ReplaceAll(input, "{{base_url}}", server.URL)
			code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll(input, "\n", nl)))
			category := "construção não suportada"
			if strings.HasPrefix(file, "invalid") {
				category = "sintaxe inválida"
			}
			if code != 2 || hits.Load() != 0 || !strings.Contains(stderr, category) || strings.Contains(out+stderr, "private") {
				t.Fatalf("file=%s code=%d hits=%d out=%s err=%s", file, code, hits.Load(), out, stderr)
			}
		}
	}
}

func TestRegexAssertionPublishedContract(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
		}
		for _, term := range []string{`regex "verificação é: ([0-9]{6})" matches "^[0-9]{6}$"`, "scenario_regex_assertion_signup.httpgo", "Extração permanece estática", "sem expansão recursiva", "--secret signup_code", "Sem query variable, body matches"} {
			if !strings.Contains(out, term) {
				t.Errorf("help missing %q", term)
			}
		}
	}
	source, err := os.ReadFile("../../testdata/scenarios/scenario_regex_assertion_signup.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../docs/assertions.md", "../../docs/subset.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), strings.TrimSpace(string(source))) {
			t.Errorf("%s differs from published scenario", path)
		}
	}
}
