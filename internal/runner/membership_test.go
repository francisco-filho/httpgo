package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func TestMembershipOrderAndPresence(t *testing.T) {
	for _, tc := range []struct {
		name, lines, body string
		status            int
		outcome           Outcome
	}{
		{"missing field", `jsonpath "$.missing" not exists`, `{}`, 200, Wildcard},
		{"missing index", `jsonpath "$[2]" not exists`, `[null]`, 200, Wildcard},
		{"missing membership", `jsonpath "$.missing" not contains null`, `{}`, 200, Wildcard},
		{"missing contains", `jsonpath "$.missing" contains null`, `{}`, 200, JSONMismatch},
		{"null present", `jsonpath "$.x" not exists`, `{"x":null}`, 200, JSONMismatch},
		{"root present", `jsonpath "$" not exists`, `null`, 200, JSONMismatch},
		{"empty present", `jsonpath "$" not exists`, `[]`, 200, JSONMismatch},
		{"false present", `jsonpath "$" not exists`, `false`, 200, JSONMismatch},
		{"zero present", `jsonpath "$" not exists`, `0`, 200, JSONMismatch},
		{"null incompatible", `jsonpath "$" not contains null`, `null`, 200, JSONMismatch},
		{"object incompatible", `jsonpath "$" not contains "x"`, `{"private":"x"}`, 200, JSONMismatch},
		{"first fails", "jsonpath \"$\" not contains 1\njsonpath \"$.bad\" not exists", `[1]`, 200, JSONMismatch},
		{"status first", `jsonpath "$" not exists`, `invalid`, 503, Mismatch},
		{"header first", "header \"X\" not contains \"blocked\"\njsonpath \"$\" not exists", `invalid`, 200, HeaderMismatch},
		{"json first", "jsonpath \"$\" not exists\nheader \"X\" not contains \"blocked\"", `invalid`, 200, JSONFailed},
		{"empty response", `jsonpath "$" not exists`, ``, 200, JSONFailed},
		{"trailing response", `jsonpath "$" not contains 1`, `[] []`, 200, JSONFailed},
		{"bad traversal", `jsonpath "$.x.y" not exists`, `{"x":null}`, 200, JSONFailed},
		{"bad index traversal", `jsonpath "$.x[0]" not contains 1`, `{"x":{}}`, 200, JSONFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expectation := "HTTP *"
			if tc.status != 200 {
				expectation = "HTTP 200"
			}
			s := parseCaptureScenario(t, "GET http://example.test/first\n"+expectation+"\n[Asserts]\n"+tc.lines+"\n[Captures]\nx: jsonpath \"$.absent\"\nGET http://example.test/next")
			// Successful assertions reach the deliberately failing capture, proving order.
			want := tc.outcome
			if want == Wildcard {
				want = CaptureFailed
			}
			calls := 0
			result := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: tc.status, Body: []byte(tc.body), Headers: []query.HeaderValue{{Name: "X", Value: "blocked"}}}, nil
			}), nil).Run(context.Background(), s)
			if calls != 1 || result.Entries[0].Outcome != want {
				t.Fatalf("calls=%d result=%+v", calls, result)
			}
		})
	}
}

func TestMembershipDemonstration(t *testing.T) {
	const lines = "jsonpath \"$.codes\" contains \"REQUIRED\"\njsonpath \"$.codes\" not contains \"OTHER_TENANT\"\njsonpath \"$.passwordHash\" not exists\nheader \"Set-Cookie\" not contains \"Domain\""
	for _, tc := range []struct {
		body, cookie string
		outcome      Outcome
	}{
		{`{"codes":["REQUIRED","INVALID"],"name":"Pizza"}`, "sid=opaque; Secure", Wildcard},
		{`{"codes":["INVALID"]}`, "sid=opaque", JSONMismatch},
		{`{"codes":["REQUIRED","OTHER_TENANT"]}`, "sid=opaque", JSONMismatch},
		{`{"codes":["REQUIRED"],"passwordHash":null}`, "sid=opaque", JSONMismatch},
		{`{"codes":["REQUIRED"]}`, "sid=opaque; Domain=example.test", HeaderMismatch},
	} {
		s := parseCaptureScenario(t, "GET http://example.test/first\nHTTP *\n[Asserts]\n"+lines+"\n[Captures]\nname: jsonpath \"$.name\"\nGET http://example.test/{{name}}")
		calls := 0
		result := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
			calls++
			if calls == 1 && !req.RetainBody {
				t.Fatal("JSON body not retained")
			}
			if calls == 2 && req.URL != "http://example.test/Pizza" {
				t.Fatal(req.URL)
			}
			return Response{Status: 200, Body: []byte(tc.body), Headers: []query.HeaderValue{{Name: "set-cookie", Value: tc.cookie}, {Name: "Set-Cookie", Value: "Domain=ignored"}}}, nil
		}), nil).Run(context.Background(), s)
		wantCalls := 1
		if tc.outcome == Wildcard {
			wantCalls = 2
		}
		if calls != wantCalls || result.Err != nil || result.Entries[0].Outcome != tc.outcome {
			t.Fatalf("calls=%d result=%+v", calls, result)
		}
	}
}

func TestMembershipSafeFailures(t *testing.T) {
	for _, line := range []string{`jsonpath "$" contains "expected-private"`, `jsonpath "$" not contains "actual-private"`, `jsonpath "$" not exists`, `header "Set-Cookie" not contains "actual-private"`} {
		for _, pending := range []bool{false, true} {
			c, _ := template.New(nil, []string{"token"})
			captures := ""
			if pending {
				captures = "\n[Captures]\ntoken: jsonpath \"$.missing.bad\""
			}
			s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n"+line+captures+"\nGET http://example.test/next")
			calls := 0
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(`["actual-private",{"unrelated":"body-private"}]`), Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: "actual-private"}}}, nil
			}), c, func(entry EntryResult) error {
				raw, _ := json.Marshal(entry)
				for _, secret := range []string{"actual-private", "body-private"} {
					if strings.Contains(string(raw), secret) {
						t.Fatalf("callback leaked %s: %s", secret, raw)
					}
				}
				if pending && strings.Contains(string(raw), "expected-private") {
					t.Fatal(string(raw))
				}
				if j := entry.Entry.Assertions[0].JSON; j != nil && j.Expected != nil {
					t.Fatal("retained raw expected")
				}
				return nil
			}).Run(context.Background(), s)
			if result.Err != nil || !result.Diverged || calls != 1 {
				t.Fatalf("%s pending=%v: %+v", line, pending, result)
			}
			if pending && result.Entries[0].JSONFailure != nil && result.Entries[0].JSONFailure.Actual != "[oculto]" {
				t.Fatal(result.Entries[0].JSONFailure)
			}
		}
	}
}

func TestMembershipLiteralAndHeaderSelection(t *testing.T) {
	for _, variables := range [][]string{nil, {"literal=resolved"}} {
		for _, tc := range []struct {
			values   []query.HeaderValue
			expected string
			pass     bool
		}{
			{nil, "", true}, {[]query.HeaderValue{{Name: "x", Value: ""}}, "x", true},
			{[]query.HeaderValue{{Name: "x", Value: ""}}, "", false},
			{[]query.HeaderValue{{Name: "x", Value: "safe"}, {Name: "X", Value: "forbidden"}}, "forbidden", true},
			{[]query.HeaderValue{{Name: "x", Value: "forbidden"}, {Name: "X", Value: "safe"}}, "forbidden", false},
			{[]query.HeaderValue{{Name: "x", Value: "a,b"}}, "a,b", false},
		} {
			c, _ := template.New(variables, nil)
			s := parseCaptureScenario(t, fmt.Sprintf("GET http://example.test\nHTTP *\n[Asserts]\nheader \"X\" not contains %q\njsonpath \"$\" contains \"{{literal}}\"", tc.expected))
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				return Response{Status: 200, Body: []byte(`"{{literal}}"`), Headers: tc.values}, nil
			}), c, nil).Run(context.Background(), s)
			if result.Err != nil || result.Diverged == tc.pass {
				t.Fatalf("%+v: %+v", tc, result)
			}
		}
	}
}
