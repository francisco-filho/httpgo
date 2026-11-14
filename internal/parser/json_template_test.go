package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestJSONTemplatesAndOrigin(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, body := range []string{`{{root}}`, `{"name":"{{product.name}}","price":{{product.price}},"a":{}}`, "{" + newline + `  "value": {{x}}` + newline + "}", `"HTTP 200 {{x}} GET http://test"`} {
			source := "POST http://example.test/" + newline + "  " + body + newline + "HTTP 201" + newline + "GET http://example.test/next"
			got, err := Parse("source.hurl", strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			entry := got.Entries[0]
			if len(got.Entries) != 2 || string(entry.Body) != body || entry.BodyPosition != (Position{"source.hurl", 2, 3}) || len(entry.BodyPlan.References) == 0 || entry.BodyPlan.End != len(body) || *entry.ExpectedStatus != 201 {
				t.Fatalf("scenario=%+v", got)
			}
		}
		_, err := Parse("source.hurl", strings.NewReader("GET http://example.test/"+newline+"POST http://example.test/"+newline+"{"+newline+`  "value": {{bad-name}}`+newline+"}"))
		var located *Error
		if !errors.As(err, &located) || located.Position != (Position{"source.hurl", 4, 12}) {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestLateJSONTemplateErrorsPreventScenario(t *testing.T) {
	for _, suffix := range []string{`{{x}} trailing`, `{"{{x}}":1}`, `{ {{x}}:1}`, `{"a":{{x}`, `{"a":{{x}} "b":1}`, `{{x}}` + "\nfile,external;", `{{x}}` + "\n[Asserts]", `{{ x.y() }}`} {
		got, err := Parse("safe.hurl", strings.NewReader("GET http://example.test/\nPOST http://example.test/\n"+suffix))
		if err == nil || len(got.Entries) != 0 {
			t.Fatalf("suffix=%q scenario=%+v err=%v", suffix, got, err)
		}
	}
	if _, err := Parse("safe.hurl", strings.NewReader("GET http://example.test/\n{{x}}")); err == nil {
		t.Fatal("GET accepted body")
	}
}
