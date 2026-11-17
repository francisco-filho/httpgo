package parser

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// GO-346,352: outer quoting is separate from the inner literal; positions are
// the original source position in both LF and CRLF files.
func TestFilterDeclarations(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		for _, path := range []string{`$[?(@.slug=='pizza')].id`, `$[?(@.slug == "it's \"pizza\"")].id`, `$[?(@.n == 9223372036854775809)]`, `$[?(@.x == '{{static}}')]`} {
			source := "GET http://example.test\nHTTP *\n[Asserts]\njsonpath " + strconv.Quote(path) + " exists\n[Captures]\nx: jsonpath " + strconv.Quote(path)
			s, err := Parse("filter.hurl", strings.NewReader(strings.ReplaceAll(source, "\n", nl)))
			if err != nil || !s.Entries[0].Assertions[0].JSON.Path.Expansive() || !s.Entries[0].Captures[0].Path.Expansive() {
				t.Fatalf("%s: %v", path, err)
			}
		}
		for _, path := range []string{`$[?(@.x == 'bad)]`, `$[?(@.x != 1)]`, `$[?(@.x == {})]`, `$[?(@.x == {{dynamic}})]`, `$[?(@.x[*] == 1)]`} {
			for _, capture := range []bool{false, true} {
				declaration := "[Asserts]\n  jsonpath " + strconv.Quote(path) + " exists"
				column := 13
				if capture {
					declaration = "[Captures]\n  x: jsonpath " + strconv.Quote(path)
					column = 16
				}
				s, err := Parse("late.hurl", strings.NewReader(strings.ReplaceAll("GET http://example.test/first\nGET http://example.test/late\nHTTP *\n"+declaration, "\n", nl)))
				var located *Error
				if !errors.As(err, &located) || len(s.Entries) != 0 || located.Position.Line != 5 || located.Position.Column != column || strings.Contains(err.Error(), "dynamic") {
					t.Fatalf("%s: %+v %v", path, s, err)
				}
			}
		}
	}
}
