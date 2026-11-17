package parser

import (
	"errors"
	"strings"
	"testing"
)

// GO-438: consume the whole declaration and preserve original LF/CRLF positions.
func TestCountCaptureSyntax(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		for _, tc := range []struct{ line, category string }{
			{`n: jsonpath "$" count`, ""}, {`n: jsonpath "$.items[*]" count`, ""},
			{`n: jsonpath "$.items[?(@.ok == true)].data" count`, ""},
			{`n: jsonpath "$"`, ""},
			{`n: jsonpath "$"count`, "sintaxe inválida"},
			{`n: jsonpath "$" count 1`, "construção não suportada"},
			{`n: jsonpath "$" count count`, "construção não suportada"},
			{`n: jsonpath "$" count regex "x"`, "construção não suportada"},
			{`n: jsonpath "$" count + 1`, "construção não suportada"},
			{`n: header "X-Items" count`, "construção não suportada"},
			{`n: jsonpath "$..items" count`, "construção não suportada"},
			{`n: jsonpath "$" count(1)`, "sintaxe inválida"},
		} {
			source := strings.Join([]string{"GET http://example.test/first", "GET http://example.test/late", "HTTP 200", "[Captures]", "  " + tc.line}, nl)
			s, err := Parse("count.hurl", strings.NewReader(source))
			if tc.category == "" {
				if err != nil {
					t.Fatalf("%s: %v", tc.line, err)
				}
				c := s.Entries[1].Captures[0]
				if c.Count != strings.HasSuffix(tc.line, " count") || c.QueryPosition != (Position{"count.hurl", 5, 16}) || c.Position.Column != 3 || c.Query == "" {
					t.Fatalf("%s: %+v", tc.line, c)
				}
			} else {
				var located *Error
				if !errors.As(err, &located) || located.Category != tc.category || located.Position.Line != 5 || len(s.Entries) != 0 {
					t.Fatalf("%s: scenario=%+v err=%v", tc.line, s, err)
				}
			}
		}
	}
}
