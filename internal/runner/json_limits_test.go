package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

func TestJSONLimitsOrderAndCaptures(t *testing.T) {
	for _, tc := range []struct {
		name, expectation, lines, body, captures string
		status, calls                            int
		outcome                                  Outcome
	}{
		{"status first", "HTTP 200", `jsonpath "$" > 0`, `invalid`, "", 503, 1, Mismatch},
		{"header first", "HTTP *", "header \"X\" exists\njsonpath \"$\" > 0", `invalid`, "", 200, 1, HeaderMismatch},
		{"JSON first", "HTTP *", "jsonpath \"$\" > 0\nheader \"X\" exists", `invalid`, "", 200, 1, JSONFailed},
		{"first mismatch", "HTTP *", "jsonpath \"$.total\" > 1\njsonpath \"$.total.bad\" exists", `{"total":1}`, "\n[Captures]\nid: jsonpath \"$.missing\"", 200, 1, CaptureFailed},
		{"capture only after", "HTTP 200", "jsonpath \"$.items\" count == 1\njsonpath \"$.id\" exists\njsonpath \"$.id\" >= 2\njsonpath \"$.id\" == 2", `{"id":2,"items":[null]}`, "\n[Captures]\nid: jsonpath \"$.id\"", 200, 2, Passed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := "GET http://example.test/next"
			if tc.calls == 2 {
				next = "GET http://example.test/{{id}}"
			}
			s := parseCaptureScenario(t, "GET http://example.test/first\n"+tc.expectation+"\n[Asserts]\n"+tc.lines+tc.captures+"\n"+next)
			calls, reports := 0, 0
			result := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				if reports != calls {
					t.Fatal("report must precede next request")
				}
				calls++
				if calls == 2 && req.URL != "http://example.test/2" {
					t.Fatalf("capture not published: %+v", req)
				}
				return Response{Status: tc.status, Body: []byte(tc.body)}, nil
			}), func(EntryResult) error { reports++; return nil }).Run(context.Background(), s)
			if calls != tc.calls || reports != calls || result.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d reports=%d result=%+v", calls, reports, result)
			}
		})
	}
}

func TestJSONLimitsSafeReports(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for _, line := range []string{`jsonpath "$.known_private" > 987654321`, `jsonpath "$.actual" > 987654321`, `jsonpath "$.items" count == 987654321`, `jsonpath "$.known_private" exists`} {
			c, _ := template.New([]string{`secret={"name":"known_private","n":987654321}`}, []string{"secret", "token"})
			captures := ""
			if pending {
				captures = "\n[Captures]\ntoken: jsonpath \"$\""
			}
			s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n"+line+captures+"\nGET http://example.test/next")
			calls := 0
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(`{"actual":987654321,"items":["body-private"],"unrelated":"body-private"}`)}, nil
			}), c, func(entry EntryResult) error {
				raw, _ := json.Marshal(entry)
				for _, secret := range []string{"known_private", "987654321", "body-private"} {
					if strings.Contains(string(raw), secret) {
						t.Fatalf("callback leaked %s: %s", secret, raw)
					}
				}
				return nil
			}).Run(context.Background(), s)
			if result.Err != nil || !result.Diverged || calls != 1 {
				t.Fatalf("result=%+v calls=%d", result, calls)
			}
			f := result.Entries[0].JSONFailure
			if pending && (f.Expected != "[oculto]" || f.Actual != "[oculto]") {
				t.Fatal(f)
			}
		}
	}
}

func TestJSONLimitsReporterPriorityAndImmutability(t *testing.T) {
	for _, body := range []string{`2`, `0`, `invalid`} {
		s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$\" > 1\nGET http://example.test/next")
		before, _ := json.Marshal(s)
		cause := errors.New("report failure")
		calls := 0
		result := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(body)}, nil
		}), func(EntryResult) error { return cause }).Run(context.Background(), s)
		after, _ := json.Marshal(s)
		if calls != 1 || !errors.Is(result.ReportErr, cause) || string(before) != string(after) {
			t.Fatalf("calls=%d result=%+v scenario mutated=%v", calls, result, string(before) != string(after))
		}
	}
}
