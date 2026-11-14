package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestTemplateSourcesAndPositions(t *testing.T) {
	body := `{ "unchanged": true }`
	input := "  POST {{ api.baseUrl }}/{{id}}\nX-Probe: prefix-{{ value }}-{{other}}\nHost: {{ host }}\n" + body + "\nHTTP 200"
	got, err := Parse("templates.hurl", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	e := got.Entries[0]
	if e.URL != "{{ api.baseUrl }}/{{id}}" || e.URLPosition != (Position{"templates.hurl", 1, 8}) || string(e.Body) != body || e.HeaderPositions[0] != (Position{"templates.hurl", 2, 10}) || e.Headers[1].Value != "{{ host }}" {
		t.Fatalf("entry=%+v", e)
	}
}

func TestLateTemplateErrors(t *testing.T) {
	for _, suffix := range []string{"GET {{bad-name}}", "GET {{ name", "GET http://example.test/}}", "GET {{name}} trailing", "GET {{name}}\nX-Test: {{a b}}", "POST {{name}}\n{\"{{value}}\":1}", "GET {{name}}\nX-{{name}}: value"} {
		got, err := Parse("safe.hurl", strings.NewReader("GET http://example.test/\n"+suffix))
		var located *Error
		if !errors.As(err, &located) || len(got.Entries) != 0 || located.Position.Line < 2 {
			t.Fatalf("Parse(%q)=%+v, %v", suffix, got, err)
		}
	}
	_, err := Parse("safe.hurl", strings.NewReader("GET http://example.test/\n  GET http://example.test/{{bad-name}}"))
	var located *Error
	if !errors.As(err, &located) || located.Position != (Position{"safe.hurl", 2, 27}) {
		t.Fatalf("position=%v", err)
	}
}
