package runner

import (
	"context"
	"errors"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func TestBodyAssertionOrder(t *testing.T) {
	for _, tc := range []struct {
		name, expectation, assertions, body, charset, captures string
		status                                                 int
		outcome                                                Outcome
		calls                                                  int
	}{
		{"demo", "HTTP 200", "body contains \"customer\"\nbody not contains \"passwordHash\"\nbody not contains \"trackingToken\"", "customer", "", "", 200, Passed, 2},
		{"forbidden", "HTTP 200", `body not contains "passwordHash"`, "customer passwordHash", "", "", 200, BodyMismatch, 1},
		{"empty contains", "HTTP *", `body contains ""`, "", "", "", 404, Wildcard, 2},
		{"empty not contains", "HTTP *", `body not contains ""`, "", "", "", 200, BodyMismatch, 1},
		{"latin1", "HTTP 200", `body contains "ação"`, "a\xe7\xe3o", "iso-8859-1", "", 200, Passed, 2},
		{"invalid not", "HTTP *", `body not contains "forbidden"`, "\xff", "utf-8", "", 200, BodyFailed, 1},
		{"unknown empty", "HTTP *", `body contains ""`, "", "unknown", "", 200, BodyFailed, 1},
		{"status first", "HTTP 200", `body contains ""`, "\xff", "utf-8", "", 503, Mismatch, 1},
		{"header first", "HTTP *", "header \"X\" exists\nbody contains \"\"", "\xff", "utf-8", "", 200, HeaderMismatch, 1},
		{"body first", "HTTP *", "body contains \"\"\nheader \"X\" exists", "\xff", "utf-8", "", 200, BodyFailed, 1},
		{"body before JSON", "HTTP *", "body contains \"absent\"\njsonpath \"$\" == null", "invalid JSON", "", "", 200, BodyMismatch, 1},
		{"JSON before body", "HTTP *", "jsonpath \"$\" == null\nbody contains \"absent\"", "invalid JSON", "", "", 200, JSONFailed, 1},
		{"interleaved", "HTTP *", "body contains \"{\\\"x\\\": 1}\"\njsonpath \"$.x\" == 1\nbody not contains \"no\"", `{"x": 1}`, "", "", 200, Wildcard, 2},
		{"capture fails before body", "HTTP *", `body contains "absent"`, "not JSON", "", "\n[Captures]\nx: jsonpath \"$.missing\"", 200, CaptureFailed, 1},
		{"allows capture", "HTTP *", `body contains "null"`, "null", "", "\n[Captures]\nx: jsonpath \"$\"", 200, Wildcard, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := parseCaptureScenario(t, "GET http://example.test/first\n"+tc.expectation+"\n[Asserts]\n"+tc.assertions+tc.captures+"\nGET http://example.test/next")
			calls, reports := 0, 0
			result := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				if calls != reports {
					t.Fatal("callback not delivered before next request")
				}
				calls++
				if req.RetainBody != (calls == 1) {
					t.Fatalf("retain=%v calls=%d", req.RetainBody, calls)
				}
				headers := []query.HeaderValue(nil)
				if tc.charset != "" {
					headers = []query.HeaderValue{{Name: "Content-Type", Value: "text/plain; charset=" + tc.charset}}
				}
				return Response{Status: tc.status, Body: []byte(tc.body), Headers: headers}, nil
			}), func(EntryResult) error { reports++; return nil }).Run(context.Background(), s)
			if calls != tc.calls || reports != calls || result.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d reports=%d result=%+v", calls, reports, result)
			}
			if tc.outcome == BodyFailed {
				var err *BodyError
				if !errors.As(result.Err, &err) || err.Position.Line != 4 {
					t.Fatalf("err=%v", result.Err)
				}
			}
		})
	}
}

func TestBodyExpectedNeverRenders(t *testing.T) {
	for _, variables := range [][]string{nil, {"name=resolved"}} {
		renderer, err := template.New(variables, nil)
		if err != nil {
			t.Fatal(err)
		}
		s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\nbody contains \"{{name}}\"")
		result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			return Response{Status: 200, Body: []byte("{{name}}")}, nil
		}), renderer, nil).Run(context.Background(), s)
		if result.Err != nil || result.Diverged {
			t.Fatalf("result=%+v", result)
		}
	}
}
