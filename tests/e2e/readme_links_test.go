package e2e

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// GitHub-style heading IDs and explicit anchors for local Markdown reference links.
func markdownAnchors(document string) map[string]bool {
	explicitAnchor := regexp.MustCompile(`^<a id="([^"]+)"></a>$`)
	anchors := make(map[string]bool)
	duplicates := make(map[string]int)
	fenced := false
	for line := range strings.SplitSeq(document, "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		if fenced {
			continue
		}
		if match := explicitAnchor.FindStringSubmatch(line); match != nil {
			anchors[match[1]] = true
		}
		if !strings.HasPrefix(line, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
		var slug strings.Builder
		for _, r := range strings.ToLower(heading) {
			switch {
			case r == ' ':
				slug.WriteByte('-')
			case unicode.IsLetter(r), unicode.IsNumber(r), r == '-', r == '_':
				slug.WriteRune(r)
			}
		}
		id := slug.String()
		if n := duplicates[id]; n > 0 {
			anchors[id+"-"+strconv.Itoa(n)] = true
		} else {
			anchors[id] = true
		}
		duplicates[id]++
	}
	return anchors
}

func TestMarkdownAnchors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		document string
		anchor   string
		want     bool
	}{
		{"heading", "## Configuração YAML", "configuração-yaml", true},
		{"duplicate heading", "## Guia\n## Guia", "guia-1", true},
		{"explicit anchor", "<a id=\"configuração-yaml\"></a>", "configuração-yaml", true},
		{"fenced heading", "```md\n## Guia\n```", "guia", false},
		{"fenced explicit anchor", "```html\n<a id=\"guia\"></a>\n```", "guia", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := markdownAnchors(tt.document)[tt.anchor]; got != tt.want {
				t.Errorf("markdownAnchors(%q)[%q] = %v, want %v", tt.document, tt.anchor, got, tt.want)
			}
		})
	}
}

func TestReadmeLinksAndProgression(t *testing.T) {
	root := filepath.Join("..", "..")
	path := filepath.Join(root, "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	trackingReference := regexp.MustCompile(`(?i)\bus-\d+|\bissue\s*#?\s*\d+|\bhist[oó]rias?\b`)
	if reference := trackingReference.FindString(readme); reference != "" {
		t.Errorf("README must document features without story or issue references: %q", reference)
	}
	previous := -1
	for _, heading := range []string{
		"Instalação", "Primeira execução e resultado", "Requisições e dados de entrada",
		"Variáveis e encadeamento", "Assertions e captures", "Configuração, suítes e relatórios",
		"Extensões e limites de compatibilidade", "Receitas adicionais", "Documentação e desenvolvimento",
	} {
		marker := "\n## " + heading + "\n"
		position := strings.Index(readme, marker)
		if strings.Count(readme, marker) != 1 || position <= previous {
			t.Errorf("section %q must occur once in progressive order", heading)
		}
		previous = position
	}
	links := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	for _, match := range links.FindAllStringSubmatch(readme, -1) {
		link, err := url.Parse(match[1])
		if err != nil {
			t.Errorf("invalid README link %q: %v", match[1], err)
			continue
		}
		if link.IsAbs() {
			continue // Public documentation is checked manually, never by network-dependent tests.
		}
		target := path
		if link.Path != "" {
			target = filepath.Join(root, filepath.FromSlash(link.Path))
		}
		content, err := os.ReadFile(target)
		if err != nil {
			t.Errorf("broken README link %q: %v", match[1], err)
			continue
		}
		if link.Fragment != "" && !markdownAnchors(string(content))[link.Fragment] {
			t.Errorf("missing anchor %q in %s", link.Fragment, target)
		}
	}
	code, help, stderr := run(t, "test", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help code=%d stderr=%q", code, stderr)
	}
	// HTTPGo options belong to CLI help. curl/Bash/installer commands are
	// exercised separately by tests/release, not interpreted as CLI flags.
	blocks := regexp.MustCompile("(?s)```sh\\n(.*?)\\n```")
	options := regexp.MustCompile(`--[a-z][a-z-]*`)
	for _, block := range blocks.FindAllStringSubmatch(readme, -1) {
		commands := strings.ReplaceAll(block[1], "\\\n", "")
		for _, command := range strings.Split(commands, "\n") {
			command = strings.TrimSpace(command)
			isHTTPGo := strings.HasPrefix(command, "httpgo ") ||
				(strings.HasPrefix(command, `"$HOME/`) && strings.Contains(command, `/httpgo" `))
			if !isHTTPGo {
				continue
			}
			for _, option := range options.FindAllString(command, -1) {
				if !strings.Contains(help, option) {
					t.Errorf("published option not in help: %s", option)
				}
			}
		}
	}
}
