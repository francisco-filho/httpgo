package runner

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

func TestTypedCaptureDemoAndIsolation(t *testing.T) {
	source := "POST http://example.test/login\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\nPOST http://example.test/products\nAuthorization: Bearer {{token}}\n{\"name\":\"Pizza\"}\nHTTP 201\n[Captures]\nproductId: jsonpath \"$.id\"\nGET http://example.test/products/{{productId}}\nAuthorization: Bearer {{token}}\nHTTP 200\nPOST http://example.test/echo\n{\"productId\":{{productId}}}\nHTTP 200"
	scenario := parseCaptureScenario(t, source)
	before := parseCaptureScenario(t, source)
	initial, _ := template.New(nil, []string{"token", "productId"})
	calls, reports := 0, 0
	r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
		if calls != reports {
			t.Fatal("send before report")
		}
		calls++
		switch (calls - 1) % 4 {
		case 0:
			return Response{Status: 200, Body: []byte(`{"token":"fake-token"}`)}, nil
		case 1:
			if req.Headers[0].Value != "Bearer fake-token" || string(req.Body) != `{"name":"Pizza"}` {
				t.Fatalf("%+v", req)
			}
			return Response{Status: 201, Body: []byte(`{"id":9007199254740993}`)}, nil
		case 2:
			if req.URL != "http://example.test/products/9007199254740993" || req.Headers[0].Value != "Bearer fake-token" {
				t.Fatalf("%+v", req)
			}
		case 3:
			if string(req.Body) != `{"productId":9007199254740993}` {
				t.Fatalf("%s", req.Body)
			}
		}
		return Response{Status: 200}, nil
	}), initial, func(e EntryResult) error {
		reports++
		if strings.Contains(e.DisplayURL, "9007199254740993") {
			t.Fatal("secret display exposed")
		}
		return nil
	})
	for range 2 {
		got := r.Run(context.Background(), scenario)
		if got.Err != nil || got.Summary != (Summary{4, 4, 4, 4, 0, 0}) {
			t.Fatalf("%+v", got)
		}
	}
	if !reflect.DeepEqual(scenario, before) {
		t.Fatal("scenario mutated")
	}
	for _, name := range []string{"token", "productId"} {
		got := r.Run(context.Background(), parseCaptureScenario(t, "GET http://example.test/{{"+name+"}}"))
		if got.Err == nil || got.Summary.Started != 0 || calls != 8 {
			t.Fatalf("capture leaked: %+v", got)
		}
		if _, err := initial.Render("{{" + name + "}}"); err == nil {
			t.Fatal("initial context mutated")
		}
	}
}

func TestTypedCapturesAllContexts(t *testing.T) {
	for _, raw := range []string{`null`, `true`, `false`, `9007199254740993`, `1.2300e+99`, `"null"`, `"{{literal}}"`, `"a%2Fb"`, `""`, `[]`, `{}`, `{"nested":[9007199254740993,1.2300e+99,"{{literal}}"]}`} {
		t.Run(raw, func(t *testing.T) {
			source := "GET http://example.test/capture\nHTTP 200\n[Captures]\nx: jsonpath \"$[0].value\"\nPOST http://example.test/next\n{{x}}\nHTTP 200"
			calls := 0
			got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if calls == 1 {
					return Response{Status: 200, Body: []byte(`[{"value":` + raw + `}]`)}, nil
				}
				if string(req.Body) != raw {
					t.Fatalf("got %s want %s", req.Body, raw)
				}
				return Response{Status: 200}, nil
			}), nil).Run(context.Background(), parseCaptureScenario(t, source))
			if got.Err != nil || calls != 2 || got.Summary.Validated != 2 {
				t.Fatalf("%+v calls=%d", got, calls)
			}
		})
	}
	for _, tc := range []struct{ raw, text string }{{`null`, "null"}, {`true`, "true"}, {`false`, "false"}, {`9007199254740993`, "9007199254740993"}, {`1.2300e+99`, "1.2300e+99"}, {`"a%2Fb"`, "a%2Fb"}} {
		calls := 0
		source := "GET http://example.test/capture\n[Captures]\nx: jsonpath \"$\"\nPOST http://example.test/{{x}}\nX-Value: pre{{x}}\n{\"text\":\"pre{{x}}\",\"full\":{{x}}}"
		got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
			calls++
			if calls == 1 {
				return Response{Status: 200, Body: []byte(tc.raw)}, nil
			}
			var body map[string]json.RawMessage
			var text string
			if req.URL != "http://example.test/"+tc.text || req.Headers[0].Value != "pre"+tc.text || json.Unmarshal(req.Body, &body) != nil || json.Unmarshal(body["text"], &text) != nil || text != "pre"+tc.text || string(body["full"]) != tc.raw {
				t.Fatalf("%+v", req)
			}
			return Response{Status: 200}, nil
		}), nil).Run(context.Background(), parseCaptureScenario(t, source))
		if got.Err != nil || calls != 2 {
			t.Fatalf("%+v", got)
		}
	}
}

func TestCompositeCaptureRejectionsLocated(t *testing.T) {
	for _, raw := range []string{`[]`, `{"secret":[9007199254740993]}`} {
		for _, nl := range []string{"\n", "\r\n"} {
			for _, tc := range []struct {
				next         string
				line, column int
			}{
				{"GET http://example.test/{{x}}", 5, 25},
				{"GET http://example.test/next\nX-Value: prefix{{x}}", 6, 16},
				{"POST http://example.test/next\n{\n  \"text\": \"prefix{{x}}\"\n}", 7, 18},
			} {
				source := strings.ReplaceAll("GET http://example.test/capture\nHTTP 200\n[Captures]\nx: jsonpath \"$\"\n"+tc.next+"\nGET http://example.test/last", "\n", nl)
				calls := 0
				got := New(executorFunc(func(context.Context, Request) (Response, error) {
					calls++
					return Response{Status: 200, Body: []byte(raw)}, nil
				}), nil).Run(context.Background(), parseCaptureScenario(t, source))
				var err *RenderError
				if !errors.As(got.Err, &err) || got.ErrorPosition != (parser.Position{File: "capture.hurl", Line: tc.line, Column: tc.column}) || calls != 1 || got.Summary != (Summary{3, 1, 1, 1, 0, 2}) || got.Entries[0].Outcome != Passed || got.Entries[1].Outcome != RenderFailed {
					t.Fatalf("raw=%s next=%q result=%+v err=%v calls=%d", raw, tc.next, got, err, calls)
				}
			}
		}
	}
}

func TestCapturedObjectOpaqueAndExplicitDeclaration(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		declarations := "object: jsonpath \"$\"\n"
		if explicit {
			declarations += "name: jsonpath \"$.name\"\n"
		}
		ref := "object.name"
		if explicit {
			ref = "name"
		}
		source := "GET http://example.test/one\n[Captures]\n" + declarations + "GET http://example.test/{{" + ref + "}}"
		calls := 0
		got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
			calls++
			if calls == 2 && req.URL != "http://example.test/Pizza" {
				t.Fatal(req.URL)
			}
			return Response{Status: 200, Body: []byte(`{"name":"Pizza"}`)}, nil
		}), nil).Run(context.Background(), parseCaptureScenario(t, source))
		if explicit && (got.Err != nil || calls != 2) || !explicit && (got.Err == nil || calls != 1) {
			t.Fatalf("%+v calls=%d", got, calls)
		}
	}
}
