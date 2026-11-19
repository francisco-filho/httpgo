package parser

import (
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/assertion"
)

// GO-421,423: both profiles validate the whole file and preserve locations.
func TestJSONInequalityParsing(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		for _, nl := range []string{"\n", "\r\n"} {
			for _, literal := range []string{`null`, `true`, `1e999999999999999999`, `"{{v}}"`, `[]`, `{"a":[1]}`} {
				s, err := ParseWithOptions("neq.hurl", strings.NewReader(strings.ReplaceAll("GET http://example.test\nHTTP 200\n[Asserts]\n  jsonpath \"$\" != "+literal, "\n", nl)), ParseOptions{RenderAsserts: dynamic})
				if err != nil {
					t.Fatal(err)
				}
				d := s.Entries[0].Assertions[0].JSON
				if d.Operator != assertion.JSONNotEqualOperator || d.Position.Line != 4 || d.Position.Column != 3 || (d.ExpectedTemplate != nil) != (dynamic && literal == `"{{v}}"`) {
					t.Fatalf("declaration=%+v", d)
				}
			}
			for _, tc := range []struct{ line, category string }{
				{`jsonpath "$" != {`, "sintaxe inválida"}, {`jsonpath "$" != 1 2`, "sintaxe inválida"},
				{`jsonpath "$" !=`, "sintaxe inválida"}, {`jsonpath "$." != 1`, "sintaxe inválida"},
				{`jsonpath "$[0:1]" != []`, "construção não suportada"},
				{`jsonpath "$" count != 0`, "construção não suportada"}, {`header "X" != "x"`, "construção não suportada"},
				{`body != "x"`, "construção não suportada"}, {`jsonpath "$" not == 1`, "construção não suportada"},
			} {
				source := strings.ReplaceAll("GET http://example.test/first\nGET http://example.test/late\nHTTP *\n[Asserts]\n  "+tc.line, "\n", nl)
				s, err := ParseWithOptions("late.hurl", strings.NewReader(source), ParseOptions{RenderAsserts: dynamic})
				var located *Error
				if !errors.As(err, &located) || located.Category != tc.category || located.Position.Line != 5 || len(s.Entries) != 0 {
					t.Fatalf("%s: scenario=%+v err=%v", tc.line, s, err)
				}
			}
		}
	}
	for _, dynamic := range []bool{false, true} {
		if _, err := ParseWithOptions("no-http.hurl", strings.NewReader("GET http://example.test\n[Asserts]\njsonpath \"$\" != null"), ParseOptions{RenderAsserts: dynamic}); err == nil {
			t.Fatal("accepted without HTTP")
		}
	}
}
