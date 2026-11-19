package parser

import (
	"errors"
	"strings"
	"testing"
)

// GO-490: whole-file validation and positions use original LF/CRLF bytes.
func TestRegexAssertionValidation(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		for _, decl := range []string{`regex "private[" exists`, `regex "(?=private)" exists`, `regex "(private)\\1" exists`, `regex "\d" exists`, `regex "(x)" == x`, `regex "(x)" exists extra`, `regex "(x)" matches "["`, `regex "(x)" == "x" extra`, `regex "(x)" regex "x" exists`, `variable "x" exists`, `body matches "x"`, `header "X" regex "(x)" exists`} {
			t.Run(decl+nl, func(t *testing.T) {
				s, err := Parse("late.hurl", strings.NewReader(strings.Join([]string{"GET http://example.test/first", "GET http://example.test/late", "HTTP 200", "[Asserts]", "  " + decl}, nl)))
				var located *Error
				if !errors.As(err, &located) || located.Position.Line != 5 || located.Position.Column < 3 || len(s.Entries) != 0 || strings.Contains(err.Error(), "private") {
					t.Fatalf("scenario=%+v error=%v", s, err)
				}
			})
		}
		for _, op := range []string{`== "x"`, `contains "x"`, `not contains "x"`, `matches "^x$"`, `exists`} {
			s, err := ParseWithOptions("regex.hurl", strings.NewReader(strings.Join([]string{"GET http://example.test", "HTTP 200", "[Asserts]", `  regex "({{literal}})" ` + op}, nl)), ParseOptions{RenderAsserts: true})
			if err != nil {
				t.Fatal(err)
			}
			d := s.Entries[0].Assertions[0].Regex
			if d.Pattern != "({{literal}})" || d.Extraction == nil || d.QueryPosition.Column != 3 || d.Position.Column != 25 {
				t.Fatalf("declaration=%+v", d)
			}
		}
	}
}
