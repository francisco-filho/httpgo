package e2e

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func extensionCompatibilitySection(t *testing.T, document, start, end string) string {
	t.Helper()
	before, rest, ok := strings.Cut(document, start)
	if !ok {
		t.Fatalf("missing compatibility marker %q", start)
	}
	section, after, ok := strings.Cut(rest, end)
	if !ok {
		t.Fatalf("missing compatibility end %q", end)
	}
	if strings.Contains(before+after, ".hurl") {
		t.Error("legacy extension outside compatibility section")
	}
	return section
}

func checkExtensionCompatibility(t *testing.T, section string) {
	t.Helper()
	for _, term := range []string{".httpgo", ".hurl", "sem renomeação", "conversão", "flag adicional", "mesma linguagem", "subconjunto Hurl", "integral", "migração obrigatória", "tests/01-health.httpgo", "tests/02-health.hurl", "httpgo test tests/", "dois arquivos aprovados"} {
		if !strings.Contains(strings.ToLower(section), strings.ToLower(term)) {
			t.Errorf("compatibility section missing %q", term)
		}
	}
}

func TestReadmeScenarioExtensionsAndPublishedPaths(t *testing.T) {
	publishedPath := regexp.MustCompile(`testdata/(?:scenarios|examples)/[\w/-]+\.httpgo`)
	for _, name := range []string{"README.md", "docs/subset.md", "docs/architecture.md", "docs/dev-server.md", "docs/junit.md", "docs/html.md", "docs/compatibility.md"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../..", name))
			if err != nil {
				t.Fatal(err)
			}
			document := string(data)
			switch name {
			case "README.md":
				section := extensionCompatibilitySection(t, document, "### Extensões de arquivo e compatibilidade\n", "### Recursos próprios do HTTPGo\n")
				checkExtensionCompatibility(t, section)
			case "docs/compatibility.md":
				_, section, ok := strings.Cut(document, "## Extensões de arquivo — US-38 / issue #79\n")
				if !ok {
					t.Fatal("missing extension compatibility section")
				}
				section, _, _ = strings.Cut(section, "\n## Assertions")
				checkExtensionCompatibility(t, section)
			default:
				if strings.Contains(document, ".hurl") {
					t.Error("legacy extension outside compatibility documentation")
				}
			}
			for _, path := range publishedPath.FindAllString(document, -1) {
				if _, err := os.Stat(filepath.Join("../..", path)); err != nil {
					t.Errorf("published scenario %s: %v", path, err)
				}
			}
		})
	}
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, help, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, stderr)
		}
		section := extensionCompatibilitySection(t, help, "Compatibilidade de extensões:\n", "Fim da compatibilidade de extensões.")
		checkExtensionCompatibility(t, section)
		if !strings.Contains(help, "httpgo test health.httpgo") {
			t.Error("missing default-extension help example")
		}
		for _, path := range publishedPath.FindAllString(help, -1) {
			if _, err := os.Stat(filepath.Join("../..", path)); err != nil {
				t.Errorf("help scenario %s: %v", path, err)
			}
		}
	}
}

func TestReadmePublishedScenarioCopies(t *testing.T) {
	// Preserve the already exercised legacy fixtures and prevent drift in the
	// recommended-extension examples; no differential corpus is regenerated.
	root := filepath.Join("../..", "testdata")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".httpgo" {
			return err
		}
		original := strings.TrimSuffix(path, ".httpgo") + ".hurl"
		original = strings.Replace(original, "examples"+string(filepath.Separator)+"html_suite", "scenarios"+string(filepath.Separator)+"html_suite", 1)
		original = strings.Replace(original, "examples"+string(filepath.Separator)+"basic", "suites"+string(filepath.Separator)+"basic", 1)
		legacy, err := os.ReadFile(original)
		if err != nil {
			return err
		}
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, legacy) {
			t.Errorf("published example %s differs from exercised fixture %s", path, original)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
