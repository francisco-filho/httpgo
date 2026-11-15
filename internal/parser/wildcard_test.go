package parser

import (
	"strings"
	"testing"
)

func TestWildcardExpectation(t *testing.T) {
	got, err := Parse("wild.hurl", strings.NewReader("GET http://example.test\nHTTP *\nGET http://example.test/next\nHTTP *"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range got.Entries {
		if !entry.AnyStatus || entry.ExpectedStatus != nil || entry.ExpectedPosition.Column != 1 {
			t.Fatalf("entry=%+v", entry)
		}
	}
	for _, input := range []string{"HTTP *", "GET http://example.test\nHTTP *\nHTTP 200", "GET http://example.test\nHTTP 200\nHTTP *", "GET http://example.test\nHTTP *\nX-Probe: value", "POST http://example.test\nHTTP *\n{}", "GET http://example.test\nHTTP **"} {
		if _, err := Parse("wild.hurl", strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}
