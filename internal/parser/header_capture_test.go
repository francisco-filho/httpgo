package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestHeaderCaptureLiteralAndPositions(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		for _, name := range []string{"ETag", "eTaG", "Set-Cookie"} {
			source := strings.Join([]string{"GET http://example.test/", "HTTP 200", "[Captures]", `  cache.etag: header "` + name + `"`}, nl)
			scenario, err := Parse("header.hurl", strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			got := scenario.Entries[0].Captures[0]
			if got.Kind != HeaderCapture || got.Header != name || got.Name != "cache.etag" || got.Position != (Position{"header.hurl", 4, 3}) || got.QueryPosition != (Position{"header.hurl", 4, 23}) {
				t.Fatalf("%+v", got)
			}
		}
	}
}

func TestHeaderCaptureRejectsWholeFile(t *testing.T) {
	for _, tc := range []struct{ declaration, category string }{
		{`x: header ETag`, "sintaxe inválida"},
		{`x: header ""`, "sintaxe inválida"},
		{`x: header "bad name"`, "sintaxe inválida"},
		{`x: header "{{name}}"`, "sintaxe inválida"},
		{`x: header "X:Bad"`, "sintaxe inválida"},
		{`x: header "ETag`, "sintaxe inválida"},
		{`x: header "ETag"[0]`, "sintaxe inválida"},
		{`x: header "ETag" regex "x"`, "construção não suportada"},
		{`x: header "ETag" count`, "construção não suportada"},
		{`x: header "ETag" nth 1`, "construção não suportada"},
		{`1x: header "ETag"`, "sintaxe inválida"},
	} {
		for _, nl := range []string{"\n", "\r\n"} {
			scenario, err := Parse("header.hurl", strings.NewReader(strings.ReplaceAll("GET http://example.test/first\nGET http://example.test/late\n[Captures]\n"+tc.declaration, "\n", nl)))
			var located *Error
			if !errors.As(err, &located) || len(scenario.Entries) != 0 || located.Category != tc.category || located.Position.Line != 4 || strings.Contains(err.Error(), tc.declaration) {
				t.Fatalf("%q: scenario=%+v err=%v", tc.declaration, scenario, err)
			}
		}
	}
}
