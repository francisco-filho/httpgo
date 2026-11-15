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
	for _, seed := range []string{
		"GET http://example.test/first\nHTTP 200\nPOST http://example.test/products\nfile,../payloads/product_template.json;\nHTTP 201",
		"GET http://example.test/capture\r\nHTTP 200\r\n[Captures]\r\nproduct: jsonpath \"$\"\r\nPOST http://example.test/echo\r\nfile, {{literal}} payload.data ;\r\nHTTP 201\r\nPOST http://example.test/upload\r\n[MultipartFormData]\r\ndocument: file,upload.bin;",
	} {
		f.Add(seed)
	}
	for _, form := range []string{
		"description: {{category}}\ndocument: file,../payloads/olá mundo.pdf; application/pdf",
		"document: file,{{literal}}.bin;",
		"document: file,empty;\nafter: {\"x\":true}",
		"a: text", "a: file,a;\na: text", "a: file,a;\nb: file,b;",
		"{{dynamic}}: file,a;", "a: file,a; application/pdf\rInjected: bad",
	} {
		f.Add("POST http://example.test\n[MultipartFormData]\n" + form + "\nHTTP 201")
	}
	for _, reference := range []string{"../payloads/a b.bin;", "{{missing}};", `C:\payloads\a.bin;`, ";", "a,b;", "a;b;", "unterminated", "a;\n{}", "a;\nX-Late: v"} {
		f.Add("POST http://example.test\r\n  file," + reference + "\r\nHTTP 201")
	}
	for _, literal := range []string{`null`, `"{{literal}}"`, `{"x":[1e999999999999999999,true]}`, `1 2`, `{{value}}`} {
		f.Add("GET http://example.test\r\nHTTP *\r\n[Asserts]\r\n  jsonpath \"$.items[0]\" == " + literal)
	}
	for _, seed := range []string{
		"GET http://example.test\nHTTP *",
		"GET http://example.test\r\nHTTP *\r\n[Asserts]\r\nheader \"X\" == \"{{literal}}\"\r\nheader \"X\" contains \"\"\r\nheader \"X\" exists",
		"GET http://example.test\nHTTP 200\n[Captures]\ntoken: jsonpath \"$\"\n[Asserts]\nheader \"Set-Cookie\" == \"private\"\nGET http://example.test/next",
		"GET http://example.test\nHTTP 200\n[Asserts]\nheader \"X\" exists\n[Captures]\ntoken: jsonpath \"$\"",
		"GET http://example.test\nHTTP *\n[Asserts]\nheader \"X\" matches \"x\"",
		"GET http://example.test\n[Asserts]",
	} {
		f.Add(seed)
	}
	for _, seed := range []string{
		"POST http://example.test/products\n{\"name\":\"Pizza\"}\nHTTP 201\n[Captures]\nid: jsonpath \"$.id\"\nGET http://example.test/products/{{id}}\nHTTP 200\nPOST http://example.test/echo\n{\"id\":{{id}}}",
		"GET http://example.test/capture\r\nHTTP 200\r\n[Captures]\r\nvalue: jsonpath \"$[0].items\"\r\nPOST http://example.test/echo\r\n{{value}}\r\nHTTP 200",
		"GET http://example.test/capture\nHTTP 200\n[Captures]\nobject: jsonpath \"$\"\nname: jsonpath \"$.name\"\nPOST http://example.test/echo\n{\"object\":{{object}},\"name\":{{name}}}",
		"GET http://example.test/capture\n[Captures]\nvalue: jsonpath \"$.array[*]\"\nPOST http://example.test/echo\n{{value}}",
	} {
		f.Add(seed)
	}
	for _, seed := range []string{"GET http://example.test/\n[Captures]\na.b: jsonpath \"$.items[0].token\"\nGET {{a.b}}", "POST http://example.test\n{\"section\":\"[Captures]\"}\nHTTP 200\n[Captures]\ntoken: jsonpath \"$\"", "GET http://example.test\r\n[Captures]\r\ntoken: jsonpath \"$..x\""} {
		f.Add(seed)
	}
	for _, body := range []string{`{{root}}`, `{"a":{},"b":{{x}}}`, "{\r\n\"a\":\"{{x}}\\n\"\r\n}", `{"{{key}}":1}`, `"bad\\{{x}}"`} {
		f.Add("POST http://example.test/\n" + body + "\nHTTP 201\nGET http://example.test/next")
	}
	for _, seed := range []string{"POST http://example.test\n{\"a\":1}\nHTTP 201", "GET http://example.test\nHTTP 200", "PATCH http://example.test\n[true,null]", "DELETE http://example.test\nAuthorization: x\rInjected: y", "POST http://example.test\n\"HTTP 200\"", "GET {{ api.baseUrl }}/{{id}}\nX-Probe: {{ value }}", "GET http://example.test/\nPOST {{base}}\n{\"a\":\"{{body}}\"}"} {
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
