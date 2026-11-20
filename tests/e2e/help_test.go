package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCompactHelpByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	t.Setenv("HTTPGO_CONFIG", "absent.yaml")
	t.Setenv("HTTPGO_ENV", "invalid/name")
	var expected string
	for _, noColor := range []string{"", "1"} {
		t.Setenv("NO_COLOR", noColor)
		for _, config := range []string{"absent", "invalid base", "unresolved process reference", "invalid environment"} {
			t.Run(noColor+"/"+config, func(t *testing.T) {
				dir := t.TempDir()
				if config != "absent" {
					// Help must not discover and execute this otherwise valid scenario.
					configFile(t, dir, "health.httpgo", "GET "+server.URL+"\nHTTP 200\n")
				}
				switch config {
				case "absent":
					// Start with an empty directory/configuration, without valid input requirements.
				case "invalid base":
					configFile(t, dir, ".httpgo.yaml", "version: [broken")
				case "unresolved process reference":
					configFile(t, dir, ".httpgo.yaml", "version: 1\nvariables:\n  endpoint: ${HTTPGO_HELP_ABSENT}\n")
				case "invalid environment":
					configFile(t, dir, ".httpgo.yaml", "version: 1\n")
					if err := os.MkdirAll(filepath.Join(dir, "environments"), 0700); err != nil {
						t.Fatal(err)
					}
					configFile(t, dir, "environments/local.yaml", "variables: [invalid")
				}
				for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
					code, out, stderr := runConfig(t, dir, args...)
					if code != 0 || stderr != "" || out == "" || hits.Load() != 0 {
						t.Fatalf("args=%v code=%d out=%q stderr=%q hits=%d", args, code, out, stderr, hits.Load())
					}
					if expected != "" && out != expected {
						t.Fatal("help varies by invocation, configuration or NO_COLOR")
					}
					expected = out
					assertCompactProcessHelp(t, out)
				}
			})
		}
	}
}

func assertCompactProcessHelp(t *testing.T, out string) {
	t.Helper()
	last := -1
	for _, section := range []string{"httpgo —", "Uso:\n", "Opções:\n", "Documentação:", "Exemplos:\n"} {
		index := strings.Index(out, section)
		if index <= last {
			t.Fatalf("missing or unordered %q in %q", section, out)
		}
		last = index
	}
	if !strings.Contains(out, "httpgo test <arquivo-ou-diretório>... [opções]") {
		t.Fatalf("usage=%q", out)
	}
	options := regexp.MustCompile(`(?m)^  (--[a-z-]+)\s+([^\n]+)$`).FindAllStringSubmatch(out, -1)
	expected := map[string][]string{
		"--help": {"ajuda"}, "--version": {"versão", "commit", "data"},
		"--config": {"arquivo", "YAML"}, "--env": {"nome", "ambiente"},
		"--env-file":  {"arquivo", "${NOME}", "uma ocorrência", "cwd"},
		"--fail-fast": {"primeira falha"}, "--output": {"junit|html", "exige --output-file"},
		"--verbose":        {"-v/-vv", "stderr", "repita", "JSON protegido", "16 KiB/body", "sem relatórios"},
		"--output-file":    {"caminho", "destino", "requer --output"},
		"--render-files":   {"arquivos externos", "templates JSON"},
		"--render-asserts": {"valores esperados", "operandos"},
		"--var":            {"nome=valor", "não sigilosa"}, "--secret": {"nome", "protegido", "sem fornecer valor secreto"},
	}
	if len(options) != len(expected) {
		t.Fatalf("options=%v", options)
	}
	for _, option := range options {
		terms, ok := expected[option[1]]
		if !ok {
			t.Errorf("unexpected option %q", option[1])
		}
		for _, term := range terms {
			if !strings.Contains(option[2], term) {
				t.Errorf("%q lacks %q", option[0], term)
			}
		}
		delete(expected, option[1])
	}
	if len(expected) != 0 {
		t.Errorf("missing options=%v", expected)
	}
	for _, term := range []string{"README.md", "docs/subset.md", "https://github.com/francisco-filho/httpgo"} {
		if !strings.Contains(out, term) {
			t.Errorf("missing docs %q", term)
		}
	}
	if !strings.HasSuffix(out, "Exemplos:\n  httpgo test health.httpgo\n  httpgo test tests/ --env local\n  httpgo test tests/profile.httpgo --env-file .env.local\n  httpgo test tests/ --output junit --output-file target/results.xml\n") {
		t.Errorf("examples are not final: %q", out)
	}
	for _, term := range []string{"[Captures]", "[Asserts]", "GET http", "HTTP 200", "version: 1", "--var token=", "--var password=", "\x1b"} {
		if strings.Contains(out, term) {
			t.Errorf("help contains tutorial, secret or escape %q", term)
		}
	}
}
