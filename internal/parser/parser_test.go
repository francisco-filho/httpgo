package parser

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSequence(t *testing.T) {
	for _, status := range []int{100, 200, 404, 500, 599} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			input := fmt.Sprintf("# comment\r\n\n  GET https://user:password@example.test/health?q=secret#fragment\n # another comment\n HTTP %d\nGET http://example.test/next\n", status)
			got, err := Parse("health.hurl", strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Entries) != 2 || *got.Entries[0].ExpectedStatus != status || got.Entries[1].ExpectedStatus != nil {
				t.Fatalf("entries = %+v", got.Entries)
			}
			if got.Entries[0].Position != (Position{"health.hurl", 3, 3}) || got.Entries[0].ExpectedPosition.Line != 5 {
				t.Fatalf("positions = %+v", got.Entries[0])
			}
		})
	}
}

func TestParseRejectsCompleteFile(t *testing.T) {
	cases := []string{
		"GET", "GET http://example.test # forbidden", "HTTP 200 # forbidden", "HTTP 099", "HTTP 600", "HTTP 20", "HTTP 0200", "HTTP +20", "HTTP abc", "HTTP 200\nHTTP 200",
		"GET /relative", "GET ftp://example.test", "GET http://", "GET http://:80", "GET http://example.test:65536", "GET http://example.test:", "GET http://[broken]", "GET http://::1/", "GET http://user:password@broken%zz/?secret=value", "GET http://example.test/\x00",
		"HTTP/1.1 200", "POST http://example.test", "[Asserts]", "[Captures]", "Authorization: password", "file,secret;", "{{variable}}", "GET http://example.test/{{variable}}", "??? password", "\xff", "\u00a0",
	}
	for _, suffix := range cases {
		t.Run(suffix, func(t *testing.T) {
			got, err := Parse("safe.hurl", strings.NewReader("GET http://example.test\n"+suffix))
			var diagnostic *Error
			if !errors.As(err, &diagnostic) || len(got.Entries) != 0 {
				t.Fatalf("Parse = %+v, %v; want no scenario and located error", got, err)
			}
			if diagnostic.Position.Line < 2 || !strings.Contains(err.Error(), "safe.hurl") {
				t.Fatalf("missing position: %v", err)
			}
			for _, secret := range []string{"password", "broken%zz", "secret=value", "variable"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("unsafe error: %v", err)
				}
			}
		})
	}
}

func TestEmptyAndOrphan(t *testing.T) {
	for _, input := range []string{"", "  \n# comment", "HTTP 200"} {
		if _, err := Parse("empty.hurl", strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadErrorSafeAndWrapped(t *testing.T) {
	cause := fmt.Errorf("sensitive raw error")
	_, err := Parse("input.hurl", io.MultiReader(strings.NewReader("GET http://example.test\n"), failingReader{cause}))
	if !errors.Is(err, cause) || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("error = %v", err)
	}
}

func TestUnsupportedFixtures(t *testing.T) {
	paths, err := filepath.Glob("../../testdata/scenarios/unsupported_*.hurl")
	if err != nil || len(paths) == 0 {
		t.Fatalf("fixtures = %v, %v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Parse(path, strings.NewReader(string(data)))
			var diagnostic *Error
			if !errors.As(err, &diagnostic) || diagnostic.Category != "construção não suportada" {
				t.Fatalf("fixture error = %v", err)
			}
		})
	}
}
