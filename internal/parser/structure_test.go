package parser

import (
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/assertion"
)

func TestJSONStructureDeclarations(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, line := range []string{`jsonpath "$" exists`, `jsonpath "$.items[0]" count == 0`, `jsonpath "$" count > 1`, `jsonpath "$" count >= 100000000000000000000000`, `jsonpath "$" count < 1e999999999999999999`, `jsonpath "$" count <= 1.0`, `jsonpath "$.total" > -1.25`, `jsonpath "$" >= 9007199254740993`, `jsonpath "$" < 1e-99999999999999999999`, `jsonpath "$" <= -0.000`} {
			input := "GET http://example.test\nHTTP *\n[Asserts]\n  " + line
			s, err := Parse("structure.hurl", strings.NewReader(strings.ReplaceAll(input, "\n", newline)))
			if err != nil {
				t.Fatalf("%s: %v", line, err)
			}
			got := s.Entries[0].Assertions[0].JSON
			if got.Position.Line != 4 || got.Position.Column != 3 || got.QueryPosition.Column != 13 || got.Count != strings.Contains(line, " count ") || (got.Operator == assertion.JSONExists && got.Expected != nil) {
				t.Fatalf("%s: %+v", line, got)
			}
		}
	}
}
