package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func TestHeaderCaptureLiteralStrings(t *testing.T) {
	for _, value := range []string{`"catalog-v1"`, "123", "true", "null", `{"x":1}`, "[1]", "{{missing}}", "first,whole", "", `quote"\\雪`} {
		t.Run(value, func(t *testing.T) {
			source := "GET http://example.test/catalog\nHTTP 200\n[Captures]\ncache.etag: header \"eTaG\"\nPOST http://example.test/next\nIf-None-Match: {{cache.etag}}\n{\"full\":{{cache.etag}},\"text\":\"pre{{cache.etag}}\"}\nHTTP 304"
			scenario := parseCaptureScenario(t, source)
			before := parseCaptureScenario(t, source)
			calls := 0
			got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if req.RetainBody {
					t.Fatal("header capture retains body")
				}
				if calls == 1 {
					return Response{Status: 200, Body: []byte("not JSON"), Headers: []query.HeaderValue{{Name: "ETAG", Value: value}, {Name: "etag", Value: "ignored"}}}, nil
				}
				var body map[string]string
				if req.Headers[0].Value != value || json.Unmarshal(req.Body, &body) != nil || body["full"] != value || body["text"] != "pre"+value {
					t.Fatalf("request=%+v body=%s", req, req.Body)
				}
				return Response{Status: 304}, nil
			}), nil).Run(context.Background(), scenario)
			if got.Err != nil || got.Diverged || calls != 2 || got.Summary.Validated != 2 || !reflect.DeepEqual(scenario, before) {
				t.Fatalf("%+v calls=%d", got, calls)
			}
		})
	}
}

func TestHeaderCaptureGatesAndMixedBatch(t *testing.T) {
	for _, tc := range []struct {
		name, expectation, declarations, body string
		status                                int
		headers                               []query.HeaderValue
		want                                  Outcome
		retain                                bool
	}{
		{"missing", "HTTP 200\n", `x: header "Missing"`, "not JSON", 200, nil, CaptureFailed, false},
		{"empty first", "HTTP 200\n", `x: header "X"`, "not JSON", 200, []query.HeaderValue{{Name: "x", Value: ""}, {Name: "X", Value: "later"}}, Passed, false},
		{"status first", "HTTP 200\n", `x: header "Missing"`, "not JSON", 503, nil, Mismatch, false},
		{"assertion first", "HTTP 200\n[Asserts]\nheader \"Missing\" exists\n", `x: header "Missing"`, "not JSON", 200, nil, CaptureFailed, false},
		{"assertion evaluation first", "HTTP *\n[Asserts]\njsonpath \"$\" == 1\n", `x: header "Missing"`, "not JSON", 200, nil, CaptureFailed, true},
		{"mixed success", "HTTP *\n", "x: header \"X\"\ny: jsonpath \"$\"", "42", 500, []query.HeaderValue{{Name: "X", Value: "literal"}}, Wildcard, true},
		{"mixed JSON fails", "", "x: header \"X\"\ny: jsonpath \"$.missing\"", "{}", 200, []query.HeaderValue{{Name: "X", Value: "literal"}}, CaptureFailed, true},
		{"mixed header fails", "", "y: jsonpath \"$\"\nx: header \"Missing\"", "42", 200, nil, CaptureFailed, true},
		{"batch collision", "", "x: header \"X\"\nx.child: jsonpath \"$\"", "42", 200, []query.HeaderValue{{Name: "X", Value: "literal"}}, CaptureFailed, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scenario := parseCaptureScenario(t, "GET http://example.test/first\n"+tc.expectation+"[Captures]\n"+tc.declarations+"\nGET http://example.test/next")
			calls := 0
			got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if calls == 1 && req.RetainBody != tc.retain {
					t.Fatalf("retain=%v", req.RetainBody)
				}
				return Response{Status: tc.status, Body: []byte(tc.body), Headers: tc.headers}, nil
			}), nil).Run(context.Background(), scenario)
			wantCalls := 1
			if tc.want == Passed || tc.want == Wildcard {
				wantCalls = 2
			}
			if got.Entries[0].Outcome != tc.want || calls != wantCalls {
				t.Fatalf("%+v calls=%d", got, calls)
			}
		})
	}
}

func TestHeaderCaptureAtomicPublicationAndConflicts(t *testing.T) {
	for _, declarations := range []string{
		"x: header \"X\"\ny: jsonpath \"$.missing\"",
		"y: jsonpath \"$\"\nx: header \"Missing\"",
		"x: header \"X\"\nx.child: jsonpath \"$\"",
		"x: header \"X\"\ninitial: header \"X\"\ninitial: header \"X\"",
		"x: header \"X\"\ninitial.child: header \"X\"",
		"x: header \"X\"\nprevious: header \"X\"\nprevious: header \"X\"",
		"x: header \"X\"\nprevious.child: header \"X\"",
	} {
		t.Run(declarations, func(t *testing.T) {
			renderer, _ := template.New([]string{"initial=original"}, nil)
			session := renderer.NewSession()
			if err := session.Publish([]template.Capture{{Name: "previous", Value: json.RawMessage(`"prior"`)}}); err != nil {
				t.Fatal(err)
			}
			scenario := parseCaptureScenario(t, "GET http://example.test/\n[Captures]\n"+declarations)
			err := captureResponse(session, scenario.Entries[0].Captures, Response{Body: []byte("42"), Headers: []query.HeaderValue{{Name: "X", Value: "literal"}}})
			if err == nil {
				t.Fatal("accepted failing batch")
			}
			for _, name := range []string{"x", "y"} {
				if _, err := session.Render("{{" + name + "}}"); err == nil {
					t.Fatal("partial publication")
				}
			}
			got, err2 := session.Render("{{initial}}/{{previous}}")
			if err2 != nil || got.Value != "original/prior" {
				t.Fatalf("%+v %v", got, err2)
			}
		})
	}
}

func TestSensitiveHeaderCaptureProvenanceAndPendingProtection(t *testing.T) {
	for _, name := range []string{"Set-Cookie", "AUTHORIZATION", "proxy-authorization", "cOOkie"} {
		for _, mode := range []string{"success", "capture failure", "pending assertion", "later transport"} {
			t.Run(name+mode, func(t *testing.T) {
				secret := `private"\\/雪{{missing}}`
				source := "GET http://example.test/first\nHTTP 200\n[Captures]\nx: header \"" + name + "\"\n"
				if mode == "capture failure" {
					source += "bad: header \"Missing\"\n"
				}
				if mode == "pending assertion" {
					source += "[Asserts]\nheader \"Other\" == \"unknown-private-operand\"\n"
				}
				source += "POST http://example.test/next\nX-Value: prefix{{x}}suffix\n{\"x\":{{x}}}\nHTTP 200"
				calls, reports := 0, 0
				renderer, _ := template.New(nil, nil)
				got := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
					calls++
					if calls == 1 {
						return Response{Status: 200, Headers: []query.HeaderValue{{Name: name, Value: secret}}}, nil
					}
					if req.Headers[0].Value != "prefix"+secret+"suffix" {
						t.Fatal("operational value changed")
					}
					var body map[string]string
					if json.Unmarshal(req.Body, &body) != nil || body["x"] != secret {
						t.Fatal("body changed")
					}
					if mode == "later transport" {
						return Response{}, errors.New("failure " + secret)
					}
					return Response{Status: 200}, nil
				}), renderer, func(e EntryResult) error {
					reports++
					rendered := fmt.Sprintf("%+v", e)
					for _, variant := range template.SecretVariants(secret) {
						if strings.Contains(rendered, variant) {
							t.Fatal("callback leaks secret")
						}
					}
					if mode == "pending assertion" && (e.HeaderFailure.Expected != "[oculto]" || e.HeaderFailure.Actual != "[oculto]") {
						t.Fatal("pending operands exposed")
					}
					if e.Entry.Captures != nil || e.Entry.Headers != nil || e.Entry.Body != nil {
						t.Fatal("operational data retained")
					}
					return nil
				}).Run(context.Background(), parseCaptureScenario(t, source))
				wantCalls := 2
				if mode == "capture failure" || mode == "pending assertion" {
					wantCalls = 1
				}
				if calls != wantCalls || reports != wantCalls {
					t.Fatalf("%+v calls=%d reports=%d", got, calls, reports)
				}
				if mode == "success" && (got.Err != nil || got.Diverged) {
					t.Fatalf("%+v", got)
				}
				if renderer.NewSession().IsSecret("x") {
					t.Fatal("shared dynamic declaration")
				}
			})
		}
	}
}

func TestUnknownCaptureVariantFailsExplicitly(t *testing.T) {
	renderer, _ := template.New(nil, nil)
	err := captureResponse(renderer.NewSession(), []parser.Capture{{Kind: parser.CaptureKind(99)}}, Response{})
	if err == nil || !strings.Contains(err.Detail, "não suportada") {
		t.Fatalf("%v", err)
	}
}
