package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestMembershipDeclarations(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, line := range []string{`jsonpath "$" contains "{{literal}}"`, `jsonpath "$.x[0]" not contains 1e999999999999999999999`, `jsonpath "$" contains true`, `jsonpath "$" contains null`, `jsonpath "$" not exists`, "jsonpath \"$\" not\tcontains false", `header "X" not contains "{{literal}}"`} {
			s, err := Parse("membership.hurl", strings.NewReader(strings.ReplaceAll("GET http://example.test\nHTTP *\n[Asserts]\n  "+line, "\n", newline)))
			if err != nil {
				t.Fatalf("%s: %v", line, err)
			}
			a := s.Entries[0].Assertions[0]
			if a.JSON != nil && (a.JSON.Position.Line != 4 || a.JSON.Position.Column != 3 || a.JSON.QueryPosition.Column != 13) {
				t.Fatalf("%+v", a.JSON)
			}
			if a.Header != nil && (a.Header.Position.Line != 4 || a.Header.Position.Column != 3) {
				t.Fatalf("%+v", a.Header)
			}
		}
	}
}

func TestMembershipInvalidLate(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, tc := range []struct{ line, category string }{
			{`jsonpath "$" contains []`, "sintaxe inválida"},
			{`jsonpath "$" not contains {}`, "sintaxe inválida"},
			{`jsonpath "$" contains "x" "extra"`, "sintaxe inválida"},
			{`jsonpath "$" contains "\x41"`, "sintaxe inválida"},
			{`jsonpath "$" contains {{value}}`, "sintaxe inválida"},
			{`jsonpath "$" not exists null`, "sintaxe inválida"},
			{`jsonpath "$" not`, "sintaxe inválida"},
			{`header "X" not`, "sintaxe inválida"},
			{`header "X" not contains true`, "sintaxe inválida"},
			{`jsonpath "$" not == 1`, "construção não suportada"},
			{`jsonpath "$" not matches "x"`, "construção não suportada"},
			{`jsonpath "$" count contains 1`, "construção não suportada"},
			{`jsonpath "$" count not contains 1`, "construção não suportada"},
			{`jsonpath "$" count not exists`, "construção não suportada"},
			{`jsonpath "$[*]" contains 1`, "construção não suportada"},
			{`jsonpath "$.{{name}}" not exists`, "construção não suportada"},
			{`header "X" not exists`, "construção não suportada"},
			{`header "X" not == "x"`, "construção não suportada"},
		} {
			input := "GET http://example.test/first\nGET http://example.test/late\nHTTP *\n[Asserts]\n  " + tc.line
			s, err := Parse("late.hurl", strings.NewReader(strings.ReplaceAll(input, "\n", newline)))
			var located *Error
			column := 3
			if strings.Contains(tc.line, "$[*]") || strings.Contains(tc.line, "$.{{name}}") {
				column = 13
			}
			if !errors.As(err, &located) || located.Category != tc.category || located.Position.Line != 5 || located.Position.Column != column || len(s.Entries) != 0 {
				t.Fatalf("%s: scenario=%+v err=%v", tc.line, s, err)
			}
		}
	}
	if _, err := Parse("no-http.hurl", strings.NewReader("GET http://example.test\n[Asserts]\njsonpath \"$\" not exists")); err == nil {
		t.Fatal("accepted without HTTP")
	}
}
