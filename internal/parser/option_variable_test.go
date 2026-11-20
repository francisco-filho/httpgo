package parser

import (
	"errors"
	"strings"
	"testing"
)

// GO-573–575: the option has its own scalar grammar, not CLI JSON inference.
func TestOptionVariableLiterals(t *testing.T) {
	for _, tc := range []struct{ literal, want string }{
		{"", `""`}, {`""`, `""`}, {`"001"`, `"001"`}, {"plain text", `"plain text"`},
		{"true", "true"}, {"false", "false"}, {"null", "null"}, {"+42", "42"}, {"-0", "-0"},
		{"9007199254740993", "9007199254740993"}, {"1.000000000000000001", "1.000000000000000001"},
		{`[1,true]`, `"[1,true]"`}, {`{"x":1}`, `"{\"x\":1}"`},
		{`{{other}}`, `"{{other}}"`}, {`${NAME}`, `"${NAME}"`}, {`newUuid()`, `"newUuid()"`},
		{`'single'`, `"'single'"`}, {`"a\n\t\u{96ea}\"\\\#"`, `"a\n\t雪\"\\#"`},
		{`a\:b\#c # comment`, `"a:b#c"`},
	} {
		t.Run(tc.literal, func(t *testing.T) {
			s, err := Parse("literal.hurl", strings.NewReader("POST http://example.test\n[Options]\nvariable: _name2="+tc.literal+"\nretry: 1\nretry-interval: 0\n{}\nHTTP 200"))
			if err != nil {
				t.Fatal(err)
			}
			got := s.Entries[0].Options.Variables
			if len(got) != 1 || got[0].Name != "_name2" || string(got[0].Value) != tc.want || got[0].Position.Line != 3 {
				t.Fatalf("got=%+v want=%s", got, tc.want)
			}
		})
	}
}

func TestInvalidOptionVariables(t *testing.T) {
	for _, declaration := range []string{
		"missing", "=value", "1name=value", "a.b=value", "a-b=value", "雪=value", "name =value",
		`name="unfinished`, `name="bad\q"`, `name="x" trailing`, `name="\u0041"`, `name="\u{d800}"`,
		`name=01`, `name=1.`, `name=1e3`, `name=+`, `name=truex`,
	} {
		for _, newline := range []string{"\n", "\r\n"} {
			t.Run(declaration+newline, func(t *testing.T) {
				source := "GET http://example.test/first\nGET http://example.test/late\n[Options]\n  variable: " + declaration + "\nvariable: name=valid\nHTTP 200"
				s, err := Parse("invalid.hurl", strings.NewReader(strings.ReplaceAll(source, "\n", newline)))
				var located *Error
				if !errors.As(err, &located) || located.Category != "sintaxe inválida" || located.Position.Line != 4 || located.Position.Column != 3 || len(s.Entries) != 0 || strings.Contains(err.Error(), declaration) {
					t.Fatalf("scenario=%+v err=%v", s, err)
				}
			})
		}
	}
}

func TestOptionVariableDuplicatesRetained(t *testing.T) {
	s, err := Parse("repeat.hurl", strings.NewReader("GET http://example.test\n[Options]\nvariable: x=first\nvariable: x=last\nHTTP 200"))
	if err != nil || len(s.Entries[0].Options.Variables) != 2 {
		t.Fatalf("scenario=%+v err=%v", s, err)
	}
}
