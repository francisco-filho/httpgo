package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestBodyAssertions(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, operator := range []string{"contains", "not contains", "not\tcontains"} {
			input := "GET http://example.test\nHTTP *\n[Asserts]\nheader \"X\" exists\n  body " + operator + " \"{{literal}}\\n雪\"\njsonpath \"$\" == null"
			s, err := Parse("body.hurl", strings.NewReader(strings.ReplaceAll(input, "\n", newline)))
			if err != nil {
				t.Fatal(err)
			}
			items := s.Entries[0].Assertions
			b := items[1].Body
			if len(items) != 3 || b == nil || b.Expected != "{{literal}}\n雪" || b.Position.Line != 5 || b.Position.Column != 3 || items[0].Header == nil || items[2].JSON == nil {
				t.Fatalf("items=%+v body=%+v", items, b)
			}
		}
	}
}

func TestInvalidBodyAssertions(t *testing.T) {
	for _, tc := range []struct{ line, category string }{
		{`body`, "sintaxe inválida"},
		{`body not`, "sintaxe inválida"},
		{`body contains private`, "sintaxe inválida"},
		{`body contains "private" "extra"`, "sintaxe inválida"},
		{`body contains {{private}}`, "sintaxe inválida"},
		{`body not contains`, "sintaxe inválida"},
		{`body not contains "private`, "sintaxe inválida"},
		{`body matches "private"`, "construção não suportada"},
		{`body == "private"`, "construção não suportada"},
		{`body not exists`, "construção não suportada"},
		{`body regex "private" contains "x"`, "construção não suportada"},
	} {
		for _, newline := range []string{"\n", "\r\n"} {
			input := "GET http://example.test\nHTTP 200\nGET http://example.test/late\nHTTP *\n[Asserts]\n  " + tc.line
			_, err := Parse("late.hurl", strings.NewReader(strings.ReplaceAll(input, "\n", newline)))
			var located *Error
			if !errors.As(err, &located) || located.Category != tc.category || located.Position.Line != 6 || located.Position.Column != 3 || strings.Contains(err.Error(), "private") {
				t.Fatalf("line=%q err=%v", tc.line, err)
			}
		}
	}
	_, err := Parse("no-http.hurl", strings.NewReader("GET http://example.test\n[Asserts]\nbody contains \"x\""))
	if err == nil {
		t.Fatal("assertions without HTTP accepted")
	}
}
