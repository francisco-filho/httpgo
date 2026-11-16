package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func TestJSONAssertionOrder(t *testing.T) {
	for _, tc := range []struct {
		name, expectation, assertions, body, captures string
		status                                        int
		outcome                                       Outcome
		calls, validated                              int
	}{
		{"demo", "HTTP 200", `jsonpath "$.name" == "Pizza"` + "\n" + `jsonpath "$.price" == 4500` + "\n" + `jsonpath "$.active" == true`, `{"name":"Pizza","price":4500,"active":true}`, "", 200, Passed, 2, 1},
		{"type", "HTTP 200", `jsonpath "$.price" == 4500`, `{"price":"4500"}`, "", 200, JSONMismatch, 1, 1},
		{"status first", "HTTP 200", `jsonpath "$" == null`, `invalid`, "", 503, Mismatch, 1, 0},
		{"header first", "HTTP 200", "header \"X\" exists\njsonpath \"$\" == null", `invalid`, "", 200, HeaderMismatch, 1, 1},
		{"JSON first", "HTTP 200", "jsonpath \"$\" == null\nheader \"X\" exists", `invalid`, "", 200, JSONFailed, 1, 1},
		{"first JSON stops", "HTTP *", "jsonpath \"$\" == 2\njsonpath \"$.bad\" == 1", `1`, "", 404, JSONMismatch, 1, 0},
		{"null", "HTTP *", `jsonpath "$.x" == null`, `{"x":null}`, "", 503, Wildcard, 2, 0},
		{"missing", "HTTP *", `jsonpath "$.x" == null`, `{}`, "", 200, JSONMismatch, 1, 0},
		{"index", "HTTP *", `jsonpath "$[1]" == null`, `[null]`, "", 200, JSONMismatch, 1, 0},
		{"incompatible", "HTTP *", `jsonpath "$.price.value" == null`, `{"price":1}`, "", 200, JSONFailed, 1, 0},
		{"empty", "HTTP *", `jsonpath "$" == null`, ``, "", 200, JSONFailed, 1, 0},
		{"trailing", "HTTP *", `jsonpath "$" == null`, `null {}`, "", 200, JSONFailed, 1, 0},
		{"assert before broken capture", "HTTP *", `jsonpath "$" == 2`, `1`, "\n[Captures]\nx: jsonpath \"$.missing\"", 200, JSONMismatch, 1, 0},
		{"capture after assertions", "HTTP *", `jsonpath "$" == 1`, `1`, "\n[Captures]\nx: jsonpath \"$.missing\"", 200, CaptureFailed, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := parseCaptureScenario(t, "GET http://example.test/first\n"+tc.expectation+"\n[Asserts]\n"+tc.assertions+tc.captures+"\nGET http://example.test/next")
			calls, reports := 0, 0
			result := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				if reports != calls {
					t.Fatal("result not reported before send")
				}
				calls++
				if calls == 1 && !req.RetainBody {
					t.Fatal("JSON did not retain body")
				}
				if calls == 2 && req.RetainBody {
					t.Fatal("plain request retained body")
				}
				return Response{Status: tc.status, Body: []byte(tc.body)}, nil
			}), func(EntryResult) error { reports++; return nil }).Run(context.Background(), s)
			if calls != tc.calls || reports != calls || result.Entries[0].Outcome != tc.outcome || result.Summary.Validated != tc.validated {
				t.Fatalf("calls=%d reports=%d result=%+v", calls, reports, result)
			}
			if tc.outcome == JSONFailed {
				var err *JSONError
				if !errors.As(result.Err, &err) || err.Position.Line != 4 {
					t.Fatalf("error=%v", result.Err)
				}
			}
		})
	}
}

func TestJSONMatchesStringSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, assertion, body string
		outcome               Outcome
	}{
		{"substring", `jsonpath "$.name" matches "Pizza"`, `{"name":"Mini Pizza"}`, Passed},
		{"anchored mismatch", `jsonpath "$.name" matches "^Pizza$"`, `{"name":"Mini Pizza"}`, JSONMismatch},
		{"case insensitive", `jsonpath "$.name" matches "(?i)^pizza"`, `{"name":"Pizza"}`, Passed},
		{"empty pattern", `jsonpath "$.name" matches ""`, `{"name":""}`, Passed},
		{"empty string mismatch", `jsonpath "$.name" matches "^$"`, `{"name":"x"}`, JSONMismatch},
		{"absent", `jsonpath "$.name" matches ""`, `{}`, JSONMismatch},
		{"numeric", `jsonpath "$.name" matches ""`, `{"name":3}`, JSONMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := parseCaptureScenario(t, "GET http://example.test/first\nHTTP 200\n[Asserts]\n"+tc.assertion+"\nGET http://example.test/next")
			calls := 0
			result := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), nil).Run(context.Background(), s)
			wantCalls := 1
			if tc.outcome == Passed {
				wantCalls = 2
			}
			if calls != wantCalls || result.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d outcome=%v result=%+v", calls, result.Entries[0].Outcome, result)
			}
		})
	}
}

func TestJSONLiteralAndCapturePublication(t *testing.T) {
	for _, variables := range [][]string{nil, {"name=resolved"}} {
		c, _ := template.New(variables, nil)
		s := parseCaptureScenario(t, "GET http://example.test/first\nHTTP 200\n[Asserts]\njsonpath \"$.literal\" == {\"x\":[\"{{name}}\"]}\njsonpath \"$.id\" == 9007199254740993\n[Captures]\nid: jsonpath \"$.id\"\nPOST http://example.test/{{id}}\n{\"id\":{{id}}}")
		calls := 0
		result := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
			calls++
			if calls == 2 && (req.URL != "http://example.test/9007199254740993" || string(req.Body) != `{"id":9007199254740993}`) {
				t.Fatalf("req=%+v", req)
			}
			return Response{Status: 200, Body: []byte(`{"literal":{"x":["{{name}}"]},"id":9007199254740993}`)}, nil
		}), c, nil).Run(context.Background(), s)
		if result.Err != nil || result.Diverged || calls != 2 {
			t.Fatalf("result=%+v calls=%d", result, calls)
		}
	}
}

func TestPendingSecretConcealsAssertionsAndMetadata(t *testing.T) {
	for _, line := range []string{`header "X" == "expected-private"`, `jsonpath "$.value" == "expected-private"`, `jsonpath "$.value" matches "expected-private"`} {
		for _, capturePath := range []string{"$.value", "$.missing.bad"} {
			c, _ := template.New(nil, []string{"token"})
			s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Captures]\ntoken: jsonpath \""+capturePath+"\"\n[Asserts]\n"+line+"\nGET http://example.test/next")
			calls := 0
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(`{"value":"actual-private","unrelated":"body-private"}`), Headers: []query.HeaderValue{{Name: "X", Value: "actual-private"}}}, nil
			}), c, func(entry EntryResult) error {
				if entry.Entry.Assertions[0].JSON != nil && (entry.Entry.Assertions[0].JSON.Pattern != "" || entry.Entry.Assertions[0].JSON.Compiled != nil) {
					t.Fatal("operational regex leaked into report metadata")
				}
				raw, err := json.Marshal(entry)
				if err != nil {
					t.Fatal(err)
				}
				for _, secret := range []string{"expected-private", "actual-private", "body-private"} {
					if strings.Contains(string(raw), secret) {
						t.Fatalf("callback leaked %s: %s", secret, raw)
					}
				}
				return nil
			}).Run(context.Background(), s)
			if !result.Diverged || result.Err != nil || calls != 1 {
				t.Fatalf("result=%+v calls=%d", result, calls)
			}
			entry := result.Entries[0]
			if entry.HeaderFailure != nil && (entry.HeaderFailure.Expected != "[oculto]" || entry.HeaderFailure.Actual != "[oculto]") {
				t.Fatal(entry.HeaderFailure)
			}
			if entry.JSONFailure != nil && (entry.JSONFailure.Expected != "[oculto]" || entry.JSONFailure.Actual != "[oculto]") {
				t.Fatal(entry.JSONFailure)
			}
			if s.Entries[0].Assertions[0].JSON != nil && s.Entries[0].Assertions[0].JSON.Expected != "expected-private" {
				t.Fatal("mutated scenario")
			}
		}
	}
}

func TestJSONKnownSecretsAreSanitizedBeforeCallback(t *testing.T) {
	c, _ := template.New([]string{`secret={"name":"known_private","escaped":"a\"b"}`}, []string{"secret"})
	s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$.known_private\" == {\"name\":\"known_private\",\"escaped\":\"a\\\"b\"}")
	result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
		return Response{Status: 200, Body: []byte(`{"known_private":{"name":"known_private","escaped":"a\"b","extra":true},"unused":"body-private"}`)}, nil
	}), c, func(entry EntryResult) error {
		raw, _ := json.Marshal(entry)
		if strings.Contains(string(raw), "known_private") || strings.Contains(string(raw), "body-private") {
			t.Fatalf("leaked: %s", raw)
		}
		return nil
	}).Run(context.Background(), s)
	if !result.Diverged || result.Entries[0].JSONFailure.Query != "$.[oculto]" {
		t.Fatalf("result=%+v", result)
	}
}
