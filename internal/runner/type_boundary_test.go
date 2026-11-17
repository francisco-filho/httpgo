package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-367/374: no per-item application, flattening, or partial branch success.
func TestJSONTypeReducedSelection(t *testing.T) {
	for _, tc := range []struct {
		path, operator, body string
		outcome              Outcome
	}{
		{`$[*].v`, `isString`, `[]`, JSONMismatch},
		{`$[*].v`, `isString`, `[{"v":"a"}]`, Passed},
		{`$[*].v`, `isString`, `[{"v":"a"},{"v":"b"}]`, JSONMismatch},
		{`$[*].v`, `isInteger`, `[{"v":9007199254740993}]`, Passed},
		{`$[*].v`, `isInteger`, `[{"v":1},{"v":2}]`, JSONMismatch},
		{`$[*].v`, `isBoolean`, `[{"v":false}]`, Passed},
		{`$[*].v`, `isBoolean`, `[{"v":true},{"v":false}]`, JSONMismatch},
		{`$[*].v`, `isEmpty`, `[]`, JSONMismatch},
		{`$[*].v`, `isEmpty`, `[{"v":[]}]`, Passed},
		{`$[*].v`, `isEmpty`, `[{"v":[]},{"v":[]}]`, JSONMismatch},
		{`$[*].v`, `isEmpty`, `[{"v":null},{}]`, JSONMismatch},
		{`$[*].v`, `isEmpty`, `[{"v":[]},null]`, JSONFailed},
		{`$[*][0]`, `isEmpty`, `[[],{}]`, JSONFailed},
		{`$[*][*]`, `isEmpty`, `[[[]],null]`, JSONFailed},
		{`$[?(@.key=='x')].v`, `isBoolean`, `[{"key":"x","v":false}]`, Passed},
	} {
		t.Run(tc.operator+tc.path+tc.body, func(t *testing.T) {
			s := parseCaptureScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\njsonpath \""+tc.path+"\" "+tc.operator+"\n[Captures]\nv: jsonpath \"$\"\nGET http://example.test/next")
			calls := 0
			got := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), nil).Run(context.Background(), s)
			wantCalls := 1
			if tc.outcome == Passed {
				wantCalls = 2
			}
			if calls != wantCalls || got.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d %+v", calls, got)
			}
		})
	}
}

// GO-370: protected leaves, derived categories and pending captures all conceal
// metadata before callbacks/results. Reusing the parsed scenario must be safe.
func TestJSONTypeProtectedResultsAndReuse(t *testing.T) {
	for _, tc := range []struct {
		name, expression, body, secret string
		pending                        bool
	}{
		{"known", `jsonpath "$" isInteger`, `"known-private"`, "known-private", false},
		{"collection", `jsonpath "$" isString`, `{"key":["known-private"]}`, "known-private", false},
		{"number", `jsonpath "$" isInteger`, `3.0`, "3.0", false},
		{"category", `jsonpath "$" isInteger`, `"ordinary-private"`, "string", false},
		{"query", `jsonpath "$.known_private" isString`, `{"known_private":false}`, "known_private", false},
		{"filter literal", `jsonpath "$[?(@.key=='known-\\u0070rivate')].v" isEmpty`, `[{"key":"known-private","v":[null]}]`, "known-private", false},
		{"pending", `jsonpath "$" isEmpty`, `["ordinary-private"]`, "", true},
		{"pending absence", `jsonpath "$.absent" isEmpty`, `{}`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "GET http://example.test\nHTTP *\n[Asserts]\n" + tc.expression + "\n[Captures]\nfuture: jsonpath \"$.not_available.bad\"\nGET http://example.test/never"
			s := parseCaptureScenario(t, source)
			original := s.Entries[0].Assertions[0].JSON.Path
			c, _ := template.New([]string{"token=" + tc.secret}, []string{"token"})
			if tc.pending {
				c, _ = template.New(nil, []string{"future"})
			}
			check := func(e EntryResult) error {
				declaration := e.Entry.Assertions[0].JSON
				if !reflect.DeepEqual(declaration.Path, query.Path{}) || declaration.Expected != nil || len(e.Entry.Captures) != 0 {
					t.Fatal("operational state retained")
				}
				failure := e.JSONFailure
				if failure == nil || failure.Actual != "[oculto]" || failure.Detail != "" || failure.Expected != "" {
					t.Fatalf("protected category exposed: %+v", failure)
				}
				raw, _ := json.Marshal(e)
				if tc.secret != "" && tc.secret != "string" && strings.Contains(string(raw), tc.secret) || strings.Contains(string(raw), "ordinary-private") || strings.Contains(string(raw), "u0070") {
					t.Fatalf("leaked: %s", raw)
				}
				return nil
			}
			calls := 0
			r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), c, check)
			for range 2 {
				got := r.Run(context.Background(), s)
				if got.Err != nil || !got.Diverged {
					t.Fatalf("%+v", got)
				}
				_ = check(got.Entries[0])
				if !reflect.DeepEqual(original, s.Entries[0].Assertions[0].JSON.Path) {
					t.Fatal("source mutated")
				}
			}
			if calls != 2 {
				t.Fatal(calls)
			}
		})
	}
}

// GO-376: counts and earlier results survive report failure in every outcome.
func TestJSONTypeCallbackFailure(t *testing.T) {
	for _, body := range []string{`[]`, `[null]`, `invalid`} {
		t.Run(fmt.Sprint(body), func(t *testing.T) {
			s := parseCaptureScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/check\nHTTP *\n[Asserts]\njsonpath \"$\" isEmpty\nGET http://example.test/never")
			calls, reports := 0, 0
			cause := errors.New("report failure")
			got := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(body)}, nil
			}), func(EntryResult) error {
				reports++
				if reports == 2 {
					return cause
				}
				return nil
			}).Run(context.Background(), s)
			if calls != 2 || reports != 2 || !errors.Is(got.ReportErr, cause) || got.Summary != (Summary{Total: 3, Started: 2, Completed: 2, Validated: 1, WithoutValidation: 1, NotStarted: 1}) {
				t.Fatalf("%+v", got)
			}
		})
	}
}
