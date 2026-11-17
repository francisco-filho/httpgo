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

// GO-355: inspect both structured fields (including unexported plans) and JSON
// presentation, on every exit and across scenario/session reuse. Escaped literals
// are removed even when a textual secret sanitizer cannot recognize the source.
func TestFilterPlansAndLiteralsNeverReachReports(t *testing.T) {
	for _, tc := range []struct {
		body    string
		outcome Outcome
	}{
		{`[{"key":"private-literal","v":[1,2]}]`, Passed},
		{`[{"key":"private-literal","v":[3,4]}]`, JSONMismatch},
		{`[{"key":"private-literal","v":[1,2]},null]`, JSONFailed},
		{`[{"key":"private-literal","v":[1,2],"bad":true}]`, CaptureFailed},
	} {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/pending=%v", tc.outcome, pending), func(t *testing.T) {
				source := "GET http://example.test\nHTTP 200\n[Asserts]\njsonpath \"$[?(@.key=='private-\\\\u006citeral')].v\" == [1,2]\n[Captures]\nvalue: jsonpath \"$[?(@.key=='private-\\\\u006citeral')].v\""
				if tc.outcome == CaptureFailed {
					source += "\nmissing: jsonpath \"$.missing\""
				}
				s := parseCaptureScenario(t, source)
				original := s.Entries[0].Assertions[0].JSON.Path
				c, _ := template.New([]string{"token=private-literal"}, []string{"token"})
				if pending {
					c, _ = template.New(nil, []string{"value"})
				}
				checks := 0
				check := func(e EntryResult) error {
					checks++
					a := e.Entry.Assertions[0].JSON
					if !reflect.DeepEqual(a.Path, query.Path{}) || a.Expected != nil || len(e.Entry.Captures) != 0 {
						t.Fatal("operational state retained")
					}
					text := fmt.Sprintf("%+v", e)
					raw, _ := json.Marshal(e)
					text += string(raw)
					if strings.Contains(text, "private-") || strings.Contains(text, "u006c") || strings.Contains(text, "[3,4]") || strings.Contains(text, "[1,2]") {
						t.Fatalf("unsafe presentation: %s", text)
					}
					if e.JSONFailure != nil && (e.JSONFailure.Actual != "[oculto]" || e.JSONFailure.Expected != "[oculto]") {
						t.Fatal("protected literal or pending capture did not conceal summary")
					}
					return nil
				}
				r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
					return Response{Status: 200, Body: []byte(tc.body)}, nil
				}), c, check)
				for range 2 {
					got := r.Run(context.Background(), s)
					if len(got.Entries) != 1 || got.Entries[0].Outcome != tc.outcome {
						t.Fatalf("result=%+v", got)
					}
					_ = check(got.Entries[0])
					if !reflect.DeepEqual(original, s.Entries[0].Assertions[0].JSON.Path) {
						t.Fatal("reused source mutated")
					}
				}
				if checks != 4 {
					t.Fatalf("checks=%d", checks)
				}
			})
		}
	}
}

// GO-360: callback failure interrupts after the same completed response,
// irrespective of success, divergence or strict evaluation error.
func TestFilterCallbackFailureKeepsCounts(t *testing.T) {
	for _, body := range []string{`[{"x":1,"v":true}]`, `[{"x":1,"v":false}]`, `[{"x":1,"v":true},null]`} {
		calls, reports := 0, 0
		cause := errors.New("report failure")
		s := parseCaptureScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/filter\nHTTP *\n[Asserts]\njsonpath \"$[?(@.x==1)].v\" == true\nGET http://example.test/never")
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
			t.Fatalf("result=%+v", got)
		}
	}
}
