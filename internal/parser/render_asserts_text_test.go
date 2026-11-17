package parser

import (
	"strings"
	"testing"
)

// GO-379,383,387,388: profile applies only to expected operands and supported operators.
func TestRenderAssertsTextParsing(t *testing.T) {
	for _, line := range []string{
		`body contains "prefix-{{value}}"`, `header "X" == "{{value}}"`,
		`header "X" not contains "{{value}}"`, `header "X" matches "^{{value}}$"`,
		`jsonpath "$" matches "^{{value}}$"`,
	} {
		s, err := ParseWithOptions("text.hurl", strings.NewReader("GET http://example.test\nHTTP 200\n[Asserts]\n"+line), ParseOptions{RenderAsserts: true})
		if err != nil {
			t.Fatalf("%s: %v", line, err)
		}
		a := s.Entries[0].Assertions[0]
		var plan *ExpectedTemplate
		switch {
		case a.Header != nil:
			plan = a.Header.ExpectedTemplate
		case a.JSON != nil:
			plan = a.JSON.ExpectedTemplate
		case a.Body != nil:
			plan = a.Body.ExpectedTemplate
		}
		if plan == nil || len(plan.References) != 1 {
			t.Fatalf("%s: plan=%+v", line, plan)
		}
	}
	for _, nl := range []string{"\n", "\r\n"} {
		for _, line := range []string{
			`body contains "{{broken"`, `header "X" == "{{bad-name}}"`,
			`header "X" matches "["`, `jsonpath "$" matches "(?=x)"`,
			`header "X" matches "\\{{bad-name}}"`, `header "X" matches "\q{{v}}"`,
			`body contains "{{v}}" "tail"`, `header "X" == {{v}}`,
			`body matches "{{v}}"`, `jsonpath "$" count matches "{{v}}"`,
			`header "{{name}}" == "value"`, `jsonpath "$.{{name}}" == 1`,
			`jsonpath "$" != {{v}}`, `header "X" count == {{v}}`,
		} {
			source := strings.ReplaceAll("GET http://example.test/first\nGET http://example.test/late\nHTTP *\n[Asserts]\n  "+line, "\n", nl)
			s, err := ParseWithOptions("late.hurl", strings.NewReader(source), ParseOptions{RenderAsserts: true})
			if err == nil || len(s.Entries) != 0 {
				t.Fatalf("accepted %s", line)
			}
			if e := err.(*Error); e.Position.Line != 5 || e.Position.Column != 3 && e.Position.Column != 13 {
				t.Fatalf("location: %v", e)
			}
		}
	}
}
