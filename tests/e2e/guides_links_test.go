package e2e

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGuideLinksAndOptions(t *testing.T) {
	code, help, stderr := run(t, "test", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help code=%d stderr=%q", code, stderr)
	}
	links := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	blocks := regexp.MustCompile("(?s)```sh\\n(.*?)\\n```")
	options := regexp.MustCompile(`--[a-z][a-z-]*`)
	for _, name := range []string{"configuration", "captures", "assertions"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("../../docs", name+".md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			readme, err := os.ReadFile("../../README.md")
			if err != nil || !strings.Contains(string(readme), "](docs/"+name+".md)") {
				t.Fatalf("README must link guide %s: %v", name, err)
			}
			if strings.Contains(string(data), ".hurl") {
				t.Error("usage examples must use .httpgo; legacy extension belongs in compatibility")
			}
			for _, match := range links.FindAllStringSubmatch(string(data), -1) {
				link, err := url.Parse(match[1])
				if err != nil {
					t.Errorf("invalid link %q: %v", match[1], err)
					continue
				}
				if link.IsAbs() {
					continue
				}
				target := path
				if link.Path != "" {
					target = filepath.Join(filepath.Dir(path), filepath.FromSlash(link.Path))
				}
				content, err := os.ReadFile(target)
				if err != nil {
					t.Errorf("broken link %q: %v", match[1], err)
					continue
				}
				if link.Fragment != "" && !markdownAnchors(string(content))[link.Fragment] {
					t.Errorf("missing anchor %q in %s", link.Fragment, target)
				}
			}
			for _, block := range blocks.FindAllStringSubmatch(string(data), -1) {
				for _, option := range options.FindAllString(block[1], -1) {
					if !strings.Contains(help, option) {
						t.Errorf("published option not in help: %s", option)
					}
				}
			}
		})
	}
}
