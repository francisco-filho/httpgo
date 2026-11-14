package parser

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestJSONBytesAndBoundaries(t *testing.T) {
	bodies := []string{`{"nested":{"value":{}}}`, `{"name":"Pizza","price":4500}`, "{\r\n \"escaped\": \"\\\" HTTP 200 GET [Asserts] #\",\r\n \"a\": [true, null, false]\r\n}", `[true,false,null]`, "[\n 1, {\"a\": 2}\n]", `"HTTP 200 POST https://test/ [Asserts] #"`, `42`, `-12.30e+4`, `true`, `false`, `null`, `""`}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		for _, body := range bodies {
			t.Run(method+body, func(t *testing.T) {
				input := method + " http://example.test/\nContent-Type: application/json\nfoo: bar\nX-Repeat: a\nX-Repeat: b\n\n  " + body + " \t\n\nHTTP 201\nGET http://example.test/next\nHTTP 404"
				got, err := Parse("json.hurl", strings.NewReader(input))
				if err != nil {
					t.Fatal(err)
				}
				if len(got.Entries) != 2 {
					t.Fatalf("entries=%+v", got)
				}
				entry := got.Entries[0]
				if entry.Method != method || string(entry.Body) != body || len(entry.Headers) != 4 || entry.Headers[1] != (Header{"foo", "bar"}) || *entry.ExpectedStatus != 201 || got.Entries[1].Body != nil {
					t.Fatalf("unexpected parsed entry: %+v", entry)
				}
			})
		}
	}
}

func TestRejectInvalidJSONAndHeadersSafely(t *testing.T) {
	for _, suffix := range []string{
		`{"password":"body-secret",}`, `{"password":"body-secret"`, `"body-secret`, `true false`, `01`, `null # body-secret`, "[1,\nHTTP 200", "{\"a\":\"bad\nbody-secret\"}", `{"a":1} HTTP 200`, "{}\n[]", "HTTP 200\n{}", "{}\nX-After: header-secret", "Bad Name: header-secret", ": header-secret", "X-Test : header-secret", "Authorization: header-secret\rInjected: body-secret", "Cookie: header-secret\x00", "Cookie: header-secret\x7f", "Cookie: header-secret\n injected", "X-Test: {{header-secret}}", `{"a":"{{body-secret}}"}`, "HTTP 200\nAuthorization: header-secret", "\"\xff\"", "Host: bad host", "Host: user:header-secret@example.test", "Host: example.test/path", "Host: example.test\nhost: example.test",
	} {
		t.Run(fmt.Sprintf("%q", suffix), func(t *testing.T) {
			input := "GET http://example.test/first\nPOST http://example.test/last\n" + suffix
			got, err := Parse("safe.hurl", strings.NewReader(input))
			var diagnostic *Error
			if !errors.As(err, &diagnostic) || len(got.Entries) != 0 || diagnostic.Position.Line < 3 || diagnostic.Position.Column < 1 {
				t.Fatalf("scenario=%+v,error=%v", got, err)
			}
			for _, secret := range []string{"body-secret", "header-secret"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("unsafe diagnostic: %v", err)
				}
			}
		})
	}
}

func TestHeadersOnGETAndBodylessMethods(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		got, err := Parse("headers.hurl", strings.NewReader(method+" http://example.test\nAuthorization: Bearer value\nProxy-Authorization: x\nCookie: y\nSet-Cookie: z\nHost: example.test:8080\nHTTP 404"))
		if err != nil || got.Entries[0].Method != method || len(got.Entries[0].Headers) != 5 || got.Entries[0].Body != nil {
			t.Fatalf("scenario=%+v,error=%v", got, err)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"POST http://example.test\n{\"a\":1}\nHTTP 201", "GET http://example.test\nHTTP 200", "PATCH http://example.test\n[true,null]", "DELETE http://example.test\nAuthorization: x\rInjected: y", "POST http://example.test\n\"HTTP 200\""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got, err := Parse("fuzz.hurl", strings.NewReader(input))
		if err != nil && len(got.Entries) != 0 {
			t.Fatal("invalid input returned executable entries")
		}
		if err == nil && len(got.Entries) == 0 {
			t.Fatal("accepted empty scenario")
		}
	})
}
