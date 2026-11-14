package runner

import (
	"context"
	"errors"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
)

func TestReportingFailureStopsExecution(t *testing.T) {
	transportErr := errors.New("transport failure")
	reportErr := errors.New("report failure")
	for _, tc := range []struct {
		name       string
		status     int
		expected   *int
		executeErr error
		failOn     int
		outcome    Outcome
		summary    Summary
		diverged   bool
	}{
		{"validated", 200, intPointer(200), nil, 1, Passed, Summary{3, 1, 1, 1, 0, 2}, false},
		{"unvalidated", 500, nil, nil, 1, Unvalidated, Summary{3, 1, 1, 0, 1, 2}, false},
		{"divergent", 503, intPointer(200), nil, 1, Mismatch, Summary{3, 1, 1, 0, 0, 2}, true},
		{"transport failure", 200, intPointer(200), transportErr, 1, Failed, Summary{3, 1, 0, 0, 0, 2}, false},
		{"after first report", 200, intPointer(200), nil, 2, Passed, Summary{3, 2, 2, 2, 0, 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scenario := parser.Scenario{Entries: []parser.Entry{
				{URL: "http://example.test/first", ExpectedStatus: tc.expected, Position: parser.Position{File: "input.hurl", Line: 1, Column: 1}},
				{URL: "http://example.test/second", ExpectedStatus: tc.expected, Position: parser.Position{File: "input.hurl", Line: 3, Column: 1}},
				{URL: "http://example.test/third", ExpectedStatus: tc.expected, Position: parser.Position{File: "input.hurl", Line: 5, Column: 1}},
			}}
			calls, reports := 0, 0
			executor := executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: tc.status}, tc.executeErr
			})
			result := New(executor, func(entry EntryResult) error {
				reports++
				if reports != calls || entry.Outcome != tc.outcome {
					t.Fatalf("calls=%d,reports=%d,entry=%+v", calls, reports, entry)
				}
				if reports == tc.failOn {
					return reportErr
				}
				return nil
			}).Run(context.Background(), scenario)
			if calls != tc.failOn || reports != tc.failOn || result.Summary != tc.summary || result.Diverged != tc.diverged || !errors.Is(result.ReportErr, reportErr) || !errors.Is(result.Err, tc.executeErr) {
				t.Fatalf("result=%+v,calls=%d,reports=%d", result, calls, reports)
			}
			if result.ErrorPosition != scenario.Entries[tc.failOn-1].Position || len(result.Entries) != tc.failOn {
				t.Fatalf("incorrect reporting failure position or entries: %+v", result)
			}
		})
	}
}

func TestReporterCancellationStopsNextRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls, reports := 0, 0
	executor := executorFunc(func(context.Context, Request) (Response, error) { calls++; return Response{Status: 200}, nil })
	result := New(executor, func(EntryResult) error { reports++; cancel(); return nil }).Run(ctx, parser.Scenario{Entries: []parser.Entry{{}, {}}})
	if calls != 1 || reports != 1 || !errors.Is(result.Err, context.Canceled) || result.ReportErr != nil || result.Summary.NotStarted != 1 {
		t.Fatalf("result=%+v,calls=%d,reports=%d", result, calls, reports)
	}
}
