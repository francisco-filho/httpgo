package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

func TestJSONStructurePendingSecret(t *testing.T) {
	for _, line := range []string{`jsonpath "$.missing" exists`, `jsonpath "$.items" count == 987654321`, `jsonpath "$.value" count == 987654321`} {
		c, _ := template.New(nil, []string{"token"})
		s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n"+line+"\n[Captures]\ntoken: jsonpath \"$.missing.bad\"\nGET http://example.test/next")
		calls := 0
		result := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
			calls++
			if !req.RetainBody {
				t.Fatal("JSON requires retained response")
			}
			return Response{Status: 200, Body: []byte(`{"items":["body-private"],"value":"actual-private"}`)}, nil
		}), c, func(entry EntryResult) error {
			raw, _ := json.Marshal(entry)
			for _, secret := range []string{"body-private", "actual-private", "987654321"} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("callback leaked %s: %s", secret, raw)
				}
			}
			if entry.Entry.Assertions[0].JSON.Expected != nil {
				t.Fatal("raw literal retained in report")
			}
			return nil
		}).Run(context.Background(), s)
		if result.Err != nil || !result.Diverged || calls != 1 {
			t.Fatalf("result=%+v calls=%d", result, calls)
		}
		failure := result.Entries[0].JSONFailure
		if failure.Expected != "[oculto]" || failure.Actual != "[oculto]" {
			t.Fatalf("failure=%+v", failure)
		}
		if strings.Contains(line, "count") && s.Entries[0].Assertions[0].JSON.Expected == nil {
			t.Fatal("scenario mutated")
		}
	}
}

func TestJSONStructureOrder(t *testing.T) {
	for _, tc := range []struct {
		lines, body string
		outcome     Outcome
	}{
		{"header \"X\" exists\njsonpath \"$\" exists", `invalid`, HeaderMismatch},
		{"jsonpath \"$\" exists\nheader \"X\" exists", `invalid`, JSONFailed},
		{"jsonpath \"$.missing\" exists\njsonpath \"$.x.bad\" exists", `{"x":null}`, JSONMismatch},
		{"jsonpath \"$.items\" count == 2\njsonpath \"$.x.bad\" exists", `{"items":[],"x":null}`, JSONMismatch},
	} {
		s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n"+tc.lines+"\n[Captures]\nx: jsonpath \"$.absent\"\nGET http://example.test/next")
		calls := 0
		result := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(tc.body)}, nil
		}), nil).Run(context.Background(), s)
		if calls != 1 || result.Entries[0].Outcome != tc.outcome {
			t.Fatalf("calls=%d result=%+v", calls, result)
		}
	}
}
