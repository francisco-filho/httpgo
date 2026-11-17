package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestProjectionDeclarations(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		for _, path := range []string{"$[*]", "$.categories[*].products[*].id", "$[0][*][0].x"} {
			source := "GET http://example.test\nHTTP *\n[Asserts]\n  jsonpath \"" + path + "\" exists\n[Captures]\n  x: jsonpath \"" + path + "\""
			s, err := Parse("projection.hurl", strings.NewReader(strings.ReplaceAll(source, "\n", nl)))
			if err != nil || !s.Entries[0].Assertions[0].JSON.Path.Expansive() || !s.Entries[0].Captures[0].Path.Expansive() {
				t.Fatalf("%s: %v", path, err)
			}
		}
		for _, path := range []string{"$[*", "$[**]", "$[*:]", "$.*", "$..x", "$[0:1]", "$[0,1]", "$[?(@.x)]"} {
			for _, declaration := range []string{"[Asserts]\n  jsonpath \"" + path + "\" exists", "[Captures]\n  x: jsonpath \"" + path + "\""} {
				source := "GET http://example.test/first\nGET http://example.test/late\nHTTP *\n" + declaration
				s, err := Parse("late.hurl", strings.NewReader(strings.ReplaceAll(source, "\n", nl)))
				var located *Error
				column := 13
				if strings.Contains(declaration, "[Captures]") {
					column = 16
				}
				if !errors.As(err, &located) || len(s.Entries) != 0 || located.Position.Line != 5 || located.Position.Column != column {
					t.Fatalf("%s: %+v %v", path, s, err)
				}
			}
		}
	}
}
