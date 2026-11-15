package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestCapturesAssociationAndPositions(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		input := strings.Join([]string{"POST http://example.test/login", `{"text":"[Captures] HTTP 200"}`, "HTTP 200", "[Captures]", `  auth.token: jsonpath "$.items[0].token"`, `empty: jsonpath "$"`, "GET http://example.test/{{auth.token}}", "[Captures]", `next: jsonpath "$.next"`}, nl)
		got, err := Parse("capture.hurl", strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Entries) != 2 || len(got.Entries[0].Captures) != 2 || len(got.Entries[1].Captures) != 1 {
			t.Fatalf("%+v", got)
		}
		c := got.Entries[0].Captures[0]
		if c.Name != "auth.token" || c.Position != (Position{"capture.hurl", 5, 3}) || c.QueryPosition != (Position{"capture.hurl", 5, 25}) {
			t.Fatalf("%+v", c)
		}
	}
}

func TestCapturesRejectWholeFile(t *testing.T) {
	for _, suffix := range []string{
		"[Captures]", "[Captures] extra", "[Captures]\n[Captures]", "[Captures]\nHTTP 200", "[Captures]\nGET http://example.test/next",
		`[Captures]
1bad: jsonpath "$"`, `[Captures]
a..b: jsonpath "$"`, `[Captures]
name jsonpath "$"`, `[Captures]
name: jsonpath $`, `[Captures]
name: jsonpath "$" extra`, `[Captures]
name: xpath "$"`, `[Captures]
name: jsonpath "$.*"`, `[Captures]
name: jsonpath "$..token"`, `[Captures]
name: jsonpath "$[-1]"`, `[Captures]
name: jsonpath "$[?(@.x)]"`, `[Captures]
name: jsonpath "$."`, `[Captures]
name: jsonpath "$"
[Asserts]`,
	} {
		for _, nl := range []string{"\n", "\r\n"} {
			input := strings.ReplaceAll("GET http://example.test/first\nGET http://example.test/late\n"+suffix, "\n", nl)
			got, err := Parse("safe.hurl", strings.NewReader(input))
			var located *Error
			if !errors.As(err, &located) || len(got.Entries) != 0 || located.Position.Line < 3 || located.Position.Column < 1 {
				t.Fatalf("%q: %+v %v", input, got, err)
			}
		}
	}
	if _, err := Parse("orphan", strings.NewReader("[Captures]\nx: jsonpath \"$\"")); err == nil {
		t.Fatal("accepted orphan")
	}
}
