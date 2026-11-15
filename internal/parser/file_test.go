package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestFileBodyLiteral(t *testing.T) {
	for _, path := range []string{"../payloads/a b.bin", " /tmp/with spaces ", `C:\payloads\a.bin`, `{{missing}}`, `"quoted"`} {
		got, err := Parse("source.hurl", strings.NewReader("POST http://example.test\n  file,"+path+";\nHTTP 201"))
		if err != nil {
			t.Fatal(err)
		}
		body := got.Entries[0].ExternalBody
		if body == nil || body.Path != path || body.Position != (Position{"source.hurl", 2, 3}) || got.Entries[0].Body != nil {
			t.Fatalf("body=%+v", body)
		}
	}
}

func TestInvalidFileBodyRejectsWholeScenario(t *testing.T) {
	for _, suffix := range []string{"file,;", "file,a", "file,a,b;", "file,a;b;", "file,a;\nfile,b;", "{}\nfile,a;", "file,a;\n{}", "file,a;\nX-Test: value", "HTTP 200\nfile,a;"} {
		t.Run(suffix, func(t *testing.T) {
			got, err := Parse("source.hurl", strings.NewReader("GET http://example.test/first\nPOST http://example.test\n"+suffix))
			var located *Error
			if !errors.As(err, &located) || len(got.Entries) != 0 || located.Position.Line < 3 {
				t.Fatalf("got=%+v err=%v", got, err)
			}
		})
	}
	_, err := Parse("get.hurl", strings.NewReader("GET http://example.test\nfile,a;"))
	if err == nil || !strings.Contains(err.Error(), "body em GET") {
		t.Fatalf("err=%v", err)
	}
}
