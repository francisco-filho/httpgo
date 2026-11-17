package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// GO-361: approval releases captures only after every predicate has passed.
func TestJSONTypeDemoAndCapture(t *testing.T) {
	s := parseCaptureScenario(t, "GET http://example.test/session-info\nHTTP 200\n[Asserts]\njsonpath \"$.token\" isString\njsonpath \"$.attempts\" isInteger\njsonpath \"$.active\" isBoolean\njsonpath \"$.issues\" isEmpty\n[Captures]\ntoken: jsonpath \"$.token\"\nGET http://example.test/{{token}}")
	calls := 0
	got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
		calls++
		if !req.RetainBody && calls == 1 {
			t.Fatal("JSON body not retained")
		}
		if calls == 2 && req.URL != "http://example.test/abc" {
			t.Fatal(req.URL)
		}
		return Response{Status: 200, Body: []byte(`{"token":"abc","attempts":3,"active":false,"issues":[]}`)}, nil
	}), nil).Run(context.Background(), s)
	if got.Err != nil || got.Diverged || calls != 2 {
		t.Fatalf("calls=%d %+v", calls, got)
	}
}

// GO-366, GO-369–370, GO-373–374: stop before captures and never retain values.
func TestJSONTypeOrderAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name, lines, body string
		status            int
		outcome           Outcome
	}{
		{"missing", `jsonpath "$.x" isString`, `{}`, 200, JSONMismatch},
		{"missing index", `jsonpath "$[0]" isEmpty`, `[]`, 200, JSONMismatch},
		{"type", `jsonpath "$" isString`, `{"response-private":[]}`, 200, JSONMismatch},
		{"notation", `jsonpath "$" isInteger`, `3e0`, 200, JSONMismatch},
		{"status first", `jsonpath "$" isEmpty`, `invalid`, 503, Mismatch},
		{"header first", "header \"X\" exists\njsonpath \"$\" isEmpty", `invalid`, 200, HeaderMismatch},
		{"JSON first", "jsonpath \"$\" isEmpty\nheader \"X\" exists", `invalid`, 200, JSONFailed},
		{"assertion first", "jsonpath \"$.x\" isEmpty\njsonpath \"$.y.z\" isEmpty", `{"x":false,"y":null}`, 200, JSONMismatch},
		{"traversal first", "jsonpath \"$.y.z\" isEmpty\njsonpath \"$.x\" isEmpty", `{"x":false,"y":null}`, 200, JSONFailed},
		{"index traversal", `jsonpath "$[0]" isEmpty`, `{}`, 200, JSONFailed},
		{"empty response", `jsonpath "$" isEmpty`, ``, 200, JSONFailed},
		{"additional JSON", `jsonpath "$" isEmpty`, `[] []`, 200, JSONFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := parseCaptureScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\n"+tc.lines+"\n[Captures]\nx: regex \"(.*)\"\nGET http://example.test/never")
			calls := 0
			got := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: tc.status, Body: []byte(tc.body)}, nil
			}), func(e EntryResult) error {
				raw, _ := json.Marshal(e)
				if strings.Contains(string(raw), "response-private") {
					t.Fatalf("leaked operand: %s", raw)
				}
				if e.JSONFailure != nil && e.JSONFailure.Expected != "" {
					t.Fatalf("invented unary expected: %+v", e.JSONFailure)
				}
				return nil
			}).Run(context.Background(), s)
			if calls != 1 || got.Entries[0].Outcome != tc.outcome || got.Summary.NotStarted != 1 {
				t.Fatalf("calls=%d %+v", calls, got)
			}
		})
	}
}
