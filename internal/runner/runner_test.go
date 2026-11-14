package runner

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
)

type executorFunc func(context.Context, Request) (Response, error)

func (f executorFunc) Execute(ctx context.Context, req Request) (Response, error) { return f(ctx, req) }

func TestSequence(t *testing.T) {
	transportErr := errors.New("transport failure")
	for _, tc := range []struct {
		name     string
		statuses []int
		expected []*int
		err      error
		outcomes []Outcome
		summary  Summary
		diverged bool
	}{
		{"two validations", []int{200, 404}, []*int{intPointer(200), intPointer(404)}, nil, []Outcome{Passed, Passed}, Summary{2, 2, 2, 2, 0, 0}, false},
		{"fail fast", []int{503, 404}, []*int{intPointer(200), intPointer(404)}, nil, []Outcome{Mismatch}, Summary{2, 1, 1, 0, 0, 1}, true},
		{"unvalidated 500", []int{500, 404}, []*int{nil, intPointer(404)}, nil, []Outcome{Unvalidated, Passed}, Summary{2, 2, 2, 1, 1, 0}, false},
		{"transport error", []int{200, 404}, []*int{intPointer(200), nil}, transportErr, []Outcome{Failed}, Summary{2, 1, 0, 0, 0, 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scenario := parser.Scenario{Entries: []parser.Entry{{URL: "http://example.test/first", ExpectedStatus: tc.expected[0]}, {URL: "http://example.test/second", ExpectedStatus: tc.expected[1]}}}
			var calls []string
			executor := executorFunc(func(ctx context.Context, req Request) (Response, error) {
				if ctx == nil {
					t.Fatal("missing context")
				}
				calls = append(calls, req.URL)
				return Response{tc.statuses[len(calls)-1]}, tc.err
			})
			got := New(executor).Run(context.Background(), scenario)
			if got.Summary != tc.summary || got.Diverged != tc.diverged || !errors.Is(got.Err, tc.err) {
				t.Fatalf("result = %+v; want summary %+v, diverged %v, err %v", got, tc.summary, tc.diverged, tc.err)
			}
			var outcomes []Outcome
			for i, entry := range got.Entries {
				outcomes = append(outcomes, entry.Outcome)
				if calls[i] != scenario.Entries[i].URL {
					t.Fatalf("out of order: %v", calls)
				}
				if entry.Outcome == Failed && entry.Status != 0 {
					t.Fatalf("invented status: %+v", entry)
				}
			}
			if !reflect.DeepEqual(outcomes, tc.outcomes) || len(calls) != tc.summary.Started {
				t.Fatalf("outcomes = %v, calls = %v", outcomes, calls)
			}
		})
	}
}

func TestCancellation(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(map[bool]string{true: "before first", false: "during first"}[before], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if before {
				cancel()
			}
			calls := 0
			executor := executorFunc(func(received context.Context, _ Request) (Response, error) {
				if received != ctx {
					t.Fatal("context was not propagated")
				}
				calls++
				cancel()
				return Response{}, received.Err()
			})
			got := New(executor).Run(ctx, parser.Scenario{Entries: []parser.Entry{{}, {}}})
			wantCalls := 1
			if before {
				wantCalls = 0
			}
			if !errors.Is(got.Err, context.Canceled) || calls != wantCalls || got.Summary.NotStarted != 2-wantCalls {
				t.Fatalf("result = %+v; calls = %d", got, calls)
			}
		})
	}
}

func TestCancellationBetweenEntries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	executor := executorFunc(func(context.Context, Request) (Response, error) {
		calls++
		cancel()
		return Response{Status: 200}, nil
	})
	got := New(executor).Run(ctx, parser.Scenario{Entries: []parser.Entry{{ExpectedStatus: intPointer(200)}, {}}})
	if !errors.Is(got.Err, context.Canceled) || calls != 1 || got.Summary != (Summary{2, 1, 1, 1, 0, 1}) || len(got.Entries) != 1 {
		t.Fatalf("result=%+v,calls=%d", got, calls)
	}
}

func intPointer(value int) *int { return &value }
