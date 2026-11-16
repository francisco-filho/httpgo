package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestJSONAssertions(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, literal := range []string{`"{{missing}}"`, `4500`, `true`, `null`, `[]`, `{}`, `{"a":["{{defined}}",1e999999999999999999]}`, `"\u0061\n\""`} {
			input := "GET http://example.test\nHTTP *\n[Asserts]\nheader \"X\" exists\n  jsonpath \"$.items[0].name\" == " + literal + "\nheader \"Y\" exists"
			got, err := Parse("json.hurl", strings.NewReader(strings.ReplaceAll(input, "\n", newline)))
			if err != nil {
				t.Fatal(err)
			}
			a := got.Entries[0].Assertions
			if len(a) != 3 || a[0].Header == nil || a[2].Header == nil || a[1].JSON == nil {
				t.Fatalf("assertions=%+v", a)
			}
			j := a[1].JSON
			if j.Position.Line != 5 || j.Position.Column != 3 || j.QueryPosition.Column != 13 || j.Query != "$.items[0].name" {
				t.Fatalf("JSON=%+v", j)
			}
			if literal == `"{{missing}}"` && j.Expected != "{{missing}}" {
				t.Fatalf("literal=%v", j.Expected)
			}
		}
	}
}

func TestInvalidJSONAssertions(t *testing.T) {
	for _, tc := range []struct{ line, category string }{
		{`jsonpath "$" == {`, "sintaxe inválida"},
		{`jsonpath "$" == 1 2`, "sintaxe inválida"},
		{`jsonpath "$" == {{value}}`, "sintaxe inválida"},
		{`jsonpath "$" == "\x41"`, "sintaxe inválida"},
		{`jsonpath "$." == 1`, "sintaxe inválida"},
		{`jsonpath "$.{{name}}" == 1`, "construção não suportada"},
		{`jsonpath "$[*]" == []`, "construção não suportada"},
		{`jsonpath "$[-1]" == 0`, "construção não suportada"},
		{`jsonpath "$..name" == 0`, "construção não suportada"},
		{`jsonpath "$" exists true`, "sintaxe inválida"},
		{`jsonpath "$" count == -1`, "sintaxe inválida"},
		{`jsonpath "$" count == 1.5`, "sintaxe inválida"},
		{`jsonpath "$" count == "3"`, "sintaxe inválida"},
		{`jsonpath "$" count`, "sintaxe inválida"},
		{`jsonpath "$" count exists`, "construção não suportada"},
		{`jsonpath "$" count != 1`, "construção não suportada"},
		{`jsonpath "$" > "1"`, "sintaxe inválida"},
		{`jsonpath "$" >= true`, "sintaxe inválida"},
		{`jsonpath "$" < null`, "sintaxe inválida"},
		{`jsonpath "$" <= []`, "sintaxe inválida"},
		{`jsonpath "$" > {}`, "sintaxe inválida"},
		{`jsonpath "$" > 1 2`, "sintaxe inválida"},
		{`jsonpath "$"== 1`, "sintaxe inválida"},
	} {
		t.Run(tc.line, func(t *testing.T) {
			got, err := Parse("late.hurl", strings.NewReader("GET http://example.test\nHTTP 200\nGET http://example.test/late\nHTTP *\n[Asserts]\n  "+tc.line))
			var located *Error
			if !errors.As(err, &located) || located.Category != tc.category || located.Position.Line != 6 || len(got.Entries) != 0 {
				t.Fatalf("got=%+v err=%v", got, err)
			}
		})
	}
	if _, err := Parse("no-http.hurl", strings.NewReader("GET http://example.test\n[Asserts]\njsonpath \"$\" == null")); err == nil {
		t.Fatal("accepted assertion without HTTP")
	}
}
