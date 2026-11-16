package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestRegexCaptureModelAndPositions(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		for _, declaration := range []string{`  otp: regex "é: (\\d{6})"`, `  otp: header "Set-Cookie" regex "refresh=([^;]+)"`} {
			s, err := Parse("regex.hurl", strings.NewReader(strings.Join([]string{"GET http://example.test/", "HTTP 200", "[Captures]", declaration}, nl)))
			if err != nil {
				t.Fatal(err)
			}
			got := s.Entries[0].Captures[0]
			quote := strings.LastIndex(declaration, `"`) - len(got.Pattern) - 1
			// Escaped JSON bytes, not decoded pattern bytes, define the position.
			if got.Kind == RegexCapture {
				quote = strings.Index(declaration, `"`)
			}
			if got.Position != (Position{"regex.hurl", 4, 3}) || got.PatternPosition != (Position{"regex.hurl", 4, quote + 1}) || got.Compiled == nil || got.Compiled.String() != got.Pattern {
				t.Fatalf("%+v", got)
			}
			if got.Kind == HeaderCapture && got.Header != "Set-Cookie" {
				t.Fatalf("%+v", got)
			}
		}
	}
}

func TestRegexCaptureStaticValidation(t *testing.T) {
	for _, tc := range []struct{ decl, category string }{
		{`x: regex "private["`, "sintaxe inválida"},
		{`x: regex "(?=private)"`, "construção não suportada"},
		{`x: regex "(private)\\1"`, "construção não suportada"},
		{`x: regex "\d"`, "sintaxe inválida"},
		{`x: regex "\x41"`, "sintaxe inválida"},
		{`x: regex "x" 1`, "construção não suportada"},
		{`x: regex "x" regex "y"`, "construção não suportada"},
		{`x: jsonpath "$" regex "(x)"`, "sintaxe inválida"},
		{`x: header "X"regex "(x)"`, "sintaxe inválida"},
		{`x: header "X" regex`, "sintaxe inválida"},
	} {
		for _, nl := range []string{"\n", "\r\n"} {
			t.Run(tc.decl+nl, func(t *testing.T) {
				s, err := Parse("late.hurl", strings.NewReader(strings.ReplaceAll("GET http://example.test/first\nGET http://example.test/late\n[Captures]\n"+tc.decl, "\n", nl)))
				var located *Error
				if !errors.As(err, &located) || located.Category != tc.category || located.Position.Line != 4 || len(s.Entries) != 0 || strings.Contains(err.Error(), "private") {
					t.Fatalf("scenario=%+v error=%v", s, err)
				}
			})
		}
	}
	for _, decl := range []string{`x: regex "no-group"`, `x: regex "({{literal}})"`, `x: regex "(\\d+)"`, `x: header "X" regex "()"`} {
		if _, err := Parse("valid.hurl", strings.NewReader("GET http://example.test/\n[Captures]\n"+decl)); err != nil {
			t.Fatalf("%s: %v", decl, err)
		}
	}
}
