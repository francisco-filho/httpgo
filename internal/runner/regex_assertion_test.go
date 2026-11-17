package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// GO-488/489/491/498: predicates use presence, not capture error semantics;
// reports must never receive the selected response value or operational plans.
func TestRegexAssertions(t *testing.T) {
	for _, tc := range []struct {
		pattern, body, predicate string
		pass, failed             bool
	}{
		{`(first)(second)`, "firstsecond firstother", `== "first"`, true, false},
		{`(first)(second)`, "firstsecond", `== "second"`, false, false},
		{`(private-value)`, "private-value", `contains "value"`, true, false},
		{`(private-value)`, "private-value", `contains "absent"`, false, false},
		{`(private-value)`, "private-value", `not contains "absent"`, true, false},
		{`(private-value)`, "private-value", `not contains "value"`, false, false},
		{`(private-value)`, "private-value", `matches "^private-.*$"`, true, false},
		{`(private-value)`, "private-value", `matches "^other$"`, false, false},
		{`(x)`, "absent", `exists`, false, false},
		{`(x)`, "absent", `== ""`, false, false},
		{`(x)`, "absent", `contains ""`, false, false},
		{`(x)`, "absent", `matches ""`, false, false},
		{`(x)`, "absent", `not contains ""`, true, false},
		{`(a)?b`, "b ab", `exists`, false, false},
		{`(a*)b`, "b ab", `exists`, true, false},
		{`(a*)b`, "b ab", `== ""`, true, false},
		{`(a*)b`, "b ab", `contains ""`, true, false},
		{`(a*)b`, "b ab", `not contains ""`, false, false},
		{`no-group`, "no-group", `not contains "x"`, false, true},
		{`no-group`, "no-group", `exists`, false, true},
	} {
		t.Run(tc.pattern+tc.predicate, func(t *testing.T) {
			s := parseCaptureScenario(t, fmt.Sprintf("GET http://example.test\nHTTP 200\n[Asserts]\nregex %q %s\nGET http://example.test/next", tc.pattern, tc.predicate))
			calls := 0
			inspect := func(e EntryResult) error {
				d := e.Entry.Assertions
				if len(d) > 0 && (d[0].Regex.Extraction != nil || d[0].Regex.Compiled != nil || d[0].Regex.Pattern != "" || d[0].Regex.Expected != "" || d[0].Regex.ExpectedTemplate != nil) {
					t.Fatalf("unsafe metadata=%+v", d[0].Regex)
				}
				return nil
			}
			got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if calls == 1 && !req.RetainBody {
					t.Fatal("body not retained")
				}
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), inspect).Run(context.Background(), s)
			wantCalls := 1
			if tc.pass {
				wantCalls = 2
			}
			if calls != wantCalls || (got.Err != nil) != tc.failed || got.Diverged != (!tc.pass && !tc.failed) {
				t.Fatalf("calls=%d result=%+v", calls, got)
			}
			if f := got.Entries[0].RegexFailure; f != nil && strings.Contains(fmt.Sprintf("%+v", f), "Actual:") {
				t.Fatal("selected value retained")
			}
		})
	}
}

func TestRegexAssertionDoesNotPublish(t *testing.T) {
	s := parseCaptureScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\nregex \"(value)\" exists\nGET http://example.test/{{value}}")
	calls := 0
	got := New(executorFunc(func(context.Context, Request) (Response, error) {
		calls++
		return Response{Status: 200, Body: []byte("value")}, nil
	}), nil).Run(context.Background(), s)
	if calls != 1 || got.Err == nil || got.Entries[1].Outcome != RenderFailed {
		t.Fatalf("calls=%d result=%+v", calls, got)
	}
}
