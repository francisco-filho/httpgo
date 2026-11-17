package parser

import (
	"errors"
	"strings"
	"testing"
)

// GO-368: a late invalid declaration must discard the entire scenario in LF/CRLF.
func TestJSONTypeDeclarations(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, operator := range []string{"isString", "isInteger", "isBoolean", "isEmpty"} {
			for _, prefix := range []string{"", "count ", "not "} {
				for _, tail := range []string{"", " null"} {
					input := "GET http://example.test/first\nGET http://example.test/late\nHTTP *\n[Asserts]\n  jsonpath \"$.x\" " + prefix + operator + tail
					s, err := Parse("types.hurl", strings.NewReader(strings.ReplaceAll(input, "\n", newline)))
					if prefix == "" && tail == "" {
						if err != nil {
							t.Fatal(err)
						}
						got := s.Entries[1].Assertions[0].JSON
						if got.Expected != nil || got.Count || string(got.Operator) != operator || got.Position != (Position{"types.hurl", 5, 3}) {
							t.Fatalf("%+v", got)
						}
						continue
					}
					category := "construção não suportada"
					if prefix == "" {
						category = "sintaxe inválida"
					}
					var located *Error
					if !errors.As(err, &located) || located.Category != category || located.Position != (Position{"types.hurl", 5, 3}) || len(s.Entries) != 0 {
						t.Fatalf("%s: %+v %v", input, s, err)
					}
				}
			}
		}
	}
}
