package parser

import (
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/assertion"
)

func TestParseHeaderAssertions(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, sections := range []string{"[Asserts]\n  header \"Content-Type\" contains \"application/json\"\nheader \"X-ID\" exists\nheader \"X-Literal\" == \"{{absent}}\"\n[Captures]\nx: jsonpath \"$\"", "[Captures]\nx: jsonpath \"$\"\n[Asserts]\n  header \"Content-Type\" contains \"application/json\"\nheader \"X-ID\" exists\nheader \"X-Literal\" == \"{{absent}}\""} {
			input := "GET http://example.test\nHTTP *\n" + sections + "\nGET http://example.test/next\nHTTP 200\n[Asserts]\nheader \"X-Empty\" == \"\""
			got, err := Parse("headers.hurl", strings.NewReader(strings.ReplaceAll(input, "\n", newline)))
			if err != nil {
				t.Fatal(err)
			}
			first := got.Entries[0]
			if len(first.Assertions) != 3 || len(first.Captures) != 1 || first.Assertions[0].Operator != assertion.HeaderContains || first.Assertions[0].Position.Column != 3 || first.Assertions[2].Expected != "{{absent}}" || len(got.Entries[1].Assertions) != 1 {
				t.Fatalf("scenario=%+v", got)
			}
		}
	}
}

func TestInvalidHeaderAssertions(t *testing.T) {
	for _, tc := range []struct{ suffix, category string }{
		{"[Asserts]", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\nGET http://example.test/next", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\n[Captures]\nx: jsonpath \"$\"", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\nheader \"X\" exists\n[Asserts]\nheader \"Y\" exists", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\nheader X exists", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\nheader \"Bad Name\" exists", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\nheader \"X\"", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\nheader \"X\"exists", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\nheader \"Authorization\" == secret-value", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\nheader \"X\" == \"secret-value\" extra", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\nheader \"X\" exists \"secret-value\"", "sintaxe inválida"},
		{"HTTP 200\n[Asserts]\nheader \"X\" matches \"secret-value\"", "construção não suportada"},
		{"HTTP 200\n[Asserts]\nheader \"X\" count == 1", "construção não suportada"},
		{"HTTP 200\n[Asserts]\njsonpath \"$.secret-value\" == \"x\"", "construção não suportada"},
	} {
		input := "GET http://example.test/first\nHTTP 200\nGET http://example.test/late\n" + tc.suffix
		got, err := Parse("late.hurl", strings.NewReader(input))
		var located *Error
		if !errors.As(err, &located) || located.Category != tc.category || located.Position.Line < 4 || len(got.Entries) != 0 || strings.Contains(err.Error(), "secret-value") {
			t.Fatalf("suffix=%q got=%+v err=%v", tc.suffix, got, err)
		}
	}
}
