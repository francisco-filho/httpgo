package parser

import (
	"errors"
	"strings"
	"testing"
)

// GO-274/278: matches uses JSON escapes, not the older header literal grammar.
func TestHeaderMatchesParsing(t *testing.T) {
	for _, tc := range []struct{ literal, want string }{
		{`"^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$"`, `^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$`},
		{`"^a\\s\"b\\\\c/$"`, "^a\\s\"b\\\\c/$"},
		{`"\u96ea|\\p{L}+"`, `雪|\p{L}+`},
		{`"(?i)(a|b){2,4}"`, `(?i)(a|b){2,4}`},
		{`"{{absent}}"`, `{{absent}}`},
		{`""`, ""},
	} {
		for _, newline := range []string{"\n", "\r\n"} {
			source := "GET http://example.test\nHTTP *\n[Asserts]\n  header \"Set-Cookie\" matches " + tc.literal
			s, err := Parse("matches.hurl", strings.NewReader(strings.ReplaceAll(source, "\n", newline)))
			if err != nil {
				t.Fatalf("literal=%s: %v", tc.literal, err)
			}
			h := s.Entries[0].Assertions[0].Header
			if h.Expected != tc.want || h.Compiled == nil || h.Compiled.String() != tc.want || h.Operator != "matches" || h.Position.Line != 4 || h.Position.Column != 3 {
				t.Fatalf("literal=%s header=%+v", tc.literal, h)
			}
		}
	}
	// Existing header operators still accept Go-only escapes; matches must not.
	for _, op := range []string{"==", "contains", "not contains"} {
		_, err := Parse("legacy.hurl", strings.NewReader("GET http://example.test\nHTTP 200\n[Asserts]\nheader \"X\" "+op+` "\x41"`))
		if err != nil {
			t.Fatalf("legacy %s: %v", op, err)
		}
	}
}

func TestHeaderMatchesInvalidLate(t *testing.T) {
	for _, tc := range []struct{ literal, category string }{
		{`"private["`, "sintaxe inválida"},
		{`"(?=private)"`, "construção não suportada"},
		{`"(?!private)"`, "construção não suportada"},
		{`"(?<=private)"`, "construção não suportada"},
		{`"(?<!private)"`, "construção não suportada"},
		{`"(private)\\1"`, "construção não suportada"},
		{`"\x41private"`, "sintaxe inválida"},
		{`"\qprivate"`, "sintaxe inválida"},
		{`"\\qprivate"`, "sintaxe inválida"},
		{`/private/`, "sintaxe inválida"},
		{`"private" "extra"`, "sintaxe inválida"},
	} {
		for _, newline := range []string{"\n", "\r\n"} {
			source := "GET http://example.test\nHTTP 200\nGET http://example.test/late\nHTTP *\n[Asserts]\n  header \"Set-Cookie\" matches " + tc.literal
			s, err := Parse("late.hurl", strings.NewReader(strings.ReplaceAll(source, "\n", newline)))
			var located *Error
			if !errors.As(err, &located) || located.Category != tc.category || located.Position.Line != 6 || located.Position.Column != 3 || len(s.Entries) != 0 || strings.Contains(err.Error(), "private") {
				t.Fatalf("literal=%s scenario=%+v err=%v", tc.literal, s, err)
			}
		}
	}
	for _, source := range []string{
		"GET http://example.test\n[Asserts]\nheader \"X\" matches \"x\"",
		"GET http://example.test\nHTTP *\n[Asserts]\nheader \"Bad Name\" matches \"x\"",
	} {
		if _, err := Parse("invalid.hurl", strings.NewReader(source)); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
}
