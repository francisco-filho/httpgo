package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCompactHelpContract(t *testing.T) {
	var previous string
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, args...)
		if code != 0 || stderr != "" || len(fake.calls) != 0 {
			t.Fatalf("args=%v code=%d stderr=%q calls=%v", args, code, stderr, fake.calls)
		}
		if previous != "" && out != previous {
			t.Fatal("help commands differ")
		}
		previous = out
		sections := []string{"httpgo —", "Uso:\n", "Opções:\n", "Documentação:", "Exemplos:\n"}
		last := -1
		for _, section := range sections {
			index := strings.Index(out, section)
			if index <= last {
				t.Fatalf("section %q missing or out of order: %q", section, out)
			}
			last = index
		}
		if !strings.Contains(out, "httpgo test <arquivo-ou-diretório>... [opções]") {
			t.Fatalf("usage=%q", out)
		}
		options := map[string][]string{
			"--help":           {"Exibe esta ajuda"},
			"--version":        {"versão", "commit", "data"},
			"--config":         {"arquivo", "configuração YAML"},
			"--env":            {"nome", "ambiente YAML"},
			"--fail-fast":      {"primeira falha"},
			"--output":         {"junit|html", "exige --output-file"},
			"--output-file":    {"caminho", "destino", "requer --output"},
			"--render-files":   {"bodies de arquivos externos", "templates JSON"},
			"--render-asserts": {"valores esperados", "operandos"},
			"--var":            {"nome=valor", "não sigilosa"},
			"--secret":         {"nome", "protegido", "sem fornecer valor secreto"},
		}
		listed := regexp.MustCompile(`(?m)^  (--[a-z-]+)\s+([^\n]+)$`).FindAllStringSubmatch(out, -1)
		if len(listed) != len(options) {
			t.Fatalf("options=%v", listed)
		}
		for _, line := range listed {
			terms, ok := options[line[1]]
			if !ok {
				t.Errorf("unimplemented option %q", line[1])
			}
			for _, term := range terms {
				if !strings.Contains(line[2], term) {
					t.Errorf("option %q lacks %q", line[0], term)
				}
			}
			delete(options, line[1])
		}
		if len(options) != 0 {
			t.Errorf("missing options=%v", options)
		}
		for _, term := range []string{"README.md", "docs/subset.md", "https://github.com/francisco-filho/httpgo"} {
			if !strings.Contains(out, term) {
				t.Errorf("missing documentation %q", term)
			}
		}
		examples := "Exemplos:\n  httpgo test health.httpgo\n  httpgo test tests/ --env local\n  httpgo test tests/ --output junit --output-file target/results.xml\n"
		if !strings.HasSuffix(out, examples) {
			t.Errorf("examples are not final: %q", out)
		}
		for _, forbidden := range []string{"[Asserts]", "[Captures]", "GET http", "HTTP 200", "version: 1", "--var token=", "--var password=", "\x1b"} {
			if strings.Contains(out, forbidden) {
				t.Errorf("help contains tutorial, secret or escape %q", forbidden)
			}
		}
	}
}

func TestHelpBypassesConfiguration(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HTTPGO_CONFIG", "missing.yaml")
	t.Setenv("HTTPGO_ENV", "invalid/environment")
	for _, state := range []string{"absent", "invalid base", "invalid environment"} {
		t.Run(state, func(t *testing.T) {
			if state != "absent" {
				text := "not: [valid yaml"
				if state == "invalid environment" {
					text = "version: 1\nvariables:\n  token: ${HTTPGO_HELP_MISSING_VARIABLE}\n"
				}
				if err := os.WriteFile(filepath.Join(dir, ".httpgo.yaml"), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
				fake := &fakeExecutor{}
				code, out, stderr := execute(t, context.Background(), fake, args...)
				if code != 0 || stderr != "" || out == "" || len(fake.calls) != 0 {
					t.Fatalf("code=%d out=%q stderr=%q calls=%v", code, out, stderr, fake.calls)
				}
			}
		})
	}
}

func TestHelpWriteFailures(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		for _, writer := range []io.Writer{brokenWriter{}, shortWriter{}} {
			var stderr bytes.Buffer
			fake := &fakeExecutor{}
			command := NewRootCommand(fake, BuildInfo{})
			command.SetArgs(args)
			command.SetOut(writer)
			command.SetErr(&stderr)
			if code := command.ExecuteContext(context.Background()); code != 2 || len(fake.calls) != 0 || !strings.Contains(stderr.String(), "escrever o resultado") || strings.Contains(stderr.String(), "raw-secret") {
				t.Fatalf("args=%v writer=%T code=%d stderr=%q calls=%v", args, writer, code, stderr.String(), fake.calls)
			}
		}
	}
}
