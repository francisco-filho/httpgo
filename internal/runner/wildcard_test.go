package runner

import (
	"context"
	"errors"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
)

func TestWildcardStatus(t *testing.T) {
	for _, status := range []int{200, 404, 503} {
		calls := 0
		got := New(executorFunc(func(context.Context, Request) (Response, error) { calls++; return Response{Status: status}, nil }), nil).Run(context.Background(), parser.Scenario{Entries: []parser.Entry{{AnyStatus: true}, {AnyStatus: true}}})
		if calls != 2 || got.Err != nil || got.Diverged || got.Summary != (Summary{2, 2, 2, 0, 2, 0}) || got.Entries[0].Outcome != Wildcard {
			t.Fatalf("status=%d result=%+v", status, got)
		}
	}
	cause := errors.New("no response")
	got := New(executorFunc(func(context.Context, Request) (Response, error) { return Response{}, cause }), nil).Run(context.Background(), parser.Scenario{Entries: []parser.Entry{{AnyStatus: true}, {}}})
	if !errors.Is(got.Err, cause) || got.Summary.Completed != 0 || got.Summary.NotStarted != 1 {
		t.Fatalf("result=%+v", got)
	}
}
