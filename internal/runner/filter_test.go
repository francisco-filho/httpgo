package runner

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-344,355,357–358: the same entity supplies predicate and capture values.
func TestFilterEntityAndSafeFailures(t *testing.T) {
	for _, tc := range []struct {
		body    string
		outcome Outcome
	}{
		{`[{"slug":"juice","active":false,"id":"p2"},{"slug":"pizza","active":true,"id":"p1"}]`, Passed},
		{`[{"slug":"pizza","active":true,"id":"p1"},{"slug":"juice","active":false,"id":"p2"}]`, Passed},
		{`[{"slug":"pizza","active":false,"id":"p1"},{"slug":"juice","active":true,"id":"p2"}]`, JSONMismatch},
		{`[{"slug":"pizza","active":true,"id":"p1"},null]`, CaptureFailed},
		{``, CaptureFailed}, {`[] []`, CaptureFailed}, {`[`, CaptureFailed},
	} {
		t.Run(tc.body, func(t *testing.T) {
			source := "GET http://example.test/products\nHTTP 200\n[Asserts]\njsonpath \"$[?(@.slug=='pizza')].active\" == true\n[Captures]\npizza_id: jsonpath \"$[?(@.slug=='pizza')].id\"\nGET http://example.test/products/{{pizza_id}}\nHTTP 200"
			s := parseCaptureScenario(t, source)
			original := s.Entries[0].Assertions[0].JSON.Path
			c, _ := template.New([]string{"private=pizza"}, []string{"private", "pizza_id"})
			check := func(e EntryResult) error {
				if e.Entry.Assertions != nil && !reflect.DeepEqual(e.Entry.Assertions[0].JSON.Path, query.Path{}) {
					t.Fatal("plan retained")
				}
				raw, _ := json.Marshal(e)
				for _, private := range []string{"pizza", "juice", "p1", "p2"} {
					if strings.Contains(string(raw), private) {
						t.Fatalf("unsafe report: %s", raw)
					}
				}
				return nil
			}
			calls := 0
			r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if calls%2 == 0 && tc.outcome == Passed && req.URL != "http://example.test/products/p1" {
					t.Fatalf("wrong request: %s", req.URL)
				}
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), c, check)
			got := r.Run(context.Background(), s)
			want := 1
			if tc.outcome == Passed {
				want = 2
			}
			if calls != want || got.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d result=%+v", calls, got)
			}
			for _, entry := range got.Entries {
				_ = check(entry)
			}
			if !reflect.DeepEqual(original, s.Entries[0].Assertions[0].JSON.Path) {
				t.Fatal("source mutated")
			}
		})
	}
}
