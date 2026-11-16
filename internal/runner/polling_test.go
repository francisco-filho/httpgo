package runner

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func pollingScenario(t *testing.T, input string) parser.Scenario {
	t.Helper()
	scenario, err := parser.Parse("polling.hurl", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	return scenario
}

func TestPollingClassification(t *testing.T) {
	for _, tc := range []struct {
		name, condition  string
		responses        []Response
		transportErr     error
		calls            int
		diverged, failed bool
	}{
		{name: "status recovers", responses: []Response{{Status: 503}, {Status: 200}}, calls: 2},
		{name: "status exhausts", responses: []Response{{Status: 503}}, calls: 4, diverged: true},
		{name: "json recovers", condition: "jsonpath \"$.status\" == \"READY\"", responses: []Response{{Status: 200, Body: []byte(`{"status":"PENDING"}`)}, {Status: 200, Body: []byte(`{"status":"PENDING"}`)}, {Status: 200, Body: []byte(`{"status":"READY"}`)}}, calls: 3},
		{name: "missing and type are mismatches", condition: "jsonpath \"$.status\" == \"READY\"", responses: []Response{{Status: 200, Body: []byte(`{}`)}, {Status: 200, Body: []byte(`{"status":null}`)}, {Status: 200, Body: []byte(`{"status":"READY"}`)}}, calls: 3},
		{name: "header recovers", condition: "header \"X-State\" == \"READY\"", responses: []Response{{Status: 200}, {Status: 200, Headers: []query.HeaderValue{{Name: "X-State", Value: "READY"}}}}, calls: 2},
		{name: "invalid json stops", condition: "jsonpath \"$.status\" == \"READY\"", responses: []Response{{Status: 200, Body: []byte(`{`)}}, calls: 1, failed: true},
		{name: "path evaluation stops", condition: "jsonpath \"$.status.name\" == \"READY\"", responses: []Response{{Status: 200, Body: []byte(`{"status":1}`)}}, calls: 1, failed: true},
		{name: "transport stops", responses: []Response{{}}, transportErr: errors.New("transport"), calls: 1, failed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "GET http://example.test/job\n[Options]\nretry: 3\nretry-interval: 0\nHTTP 200\n"
			if tc.condition != "" {
				input += "[Asserts]\n" + tc.condition + "\n"
			}
			scenario := pollingScenario(t, input)
			calls, reports, waits := 0, 0, 0
			r := New(executorFunc(func(context.Context, Request) (Response, error) {
				if reports != 0 {
					t.Fatal("intermediate callback")
				}
				response := tc.responses[min(calls, len(tc.responses)-1)]
				calls++
				return response, tc.transportErr
			}), func(entry EntryResult) error {
				reports++
				if entry.Attempts != tc.calls {
					t.Fatalf("entry=%+v", entry)
				}
				return nil
			})
			r.wait = func(_ context.Context, d time.Duration) error {
				waits++
				if d != 0 {
					t.Fatalf("interval=%v", d)
				}
				return nil
			}
			result := r.Run(context.Background(), scenario)
			if calls != tc.calls || waits != calls-1 || reports != 1 || len(result.Entries) != 1 || result.Diverged != tc.diverged || (result.Err != nil) != tc.failed || result.Summary.Started != 1 || result.Attempts.Started != calls {
				t.Fatalf("calls=%d waits=%d reports=%d result=%+v", calls, waits, reports, result)
			}
			wantCompleted := calls
			if tc.transportErr != nil {
				wantCompleted = 0
			}
			if result.Attempts.Completed != wantCompleted || result.Attempts.StatusChecks != wantCompleted {
				t.Fatalf("attempts=%+v", result.Attempts)
			}
		})
	}
}

func TestPollingBudgetAndInterval(t *testing.T) {
	for _, tc := range []struct {
		options  string
		calls    int
		interval time.Duration
	}{
		{"", 1, time.Second}, {"[Options]\nretry: 0\n", 1, time.Second}, {"[Options]\nretry: 2\n", 3, time.Second}, {"[Options]\nretry: 2\nretry-interval: 0\n", 3, 0},
	} {
		t.Run(tc.options, func(t *testing.T) {
			calls, waits := 0, 0
			r := New(executorFunc(func(context.Context, Request) (Response, error) { calls++; return Response{Status: 503}, nil }), nil)
			r.wait = func(_ context.Context, d time.Duration) error {
				waits++
				if d != tc.interval {
					t.Fatalf("interval=%v want=%v", d, tc.interval)
				}
				return nil
			}
			got := r.Run(context.Background(), pollingScenario(t, "GET http://example.test/job\n"+tc.options+"HTTP 200\nGET http://example.test/never\n"))
			if calls != tc.calls || waits != tc.calls-1 || !got.Diverged || got.Summary != (Summary{2, 1, 1, 0, 0, 1}) {
				t.Fatalf("calls=%d waits=%d result=%+v", calls, waits, got)
			}
		})
	}
}

func TestPollingPostCapturesAndLogicalSequence(t *testing.T) {
	scenario := pollingScenario(t, `POST http://example.test/before
HTTP 201
[Captures]
job: jsonpath "$.id"
POST http://example.test/jobs/{{job}}
X-Job: {{job}}
[Options]
retry: 3
retry-interval: 0
{"id":"{{job}}"}
HTTP 200
[Captures]
ready: jsonpath "$.ready"
[Asserts]
jsonpath "$.status" == "READY"
GET http://example.test/{{ready}}
HTTP 200
`)
	var requests []Request
	reports := 0
	r := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
		requests = append(requests, req)
		switch len(requests) {
		case 1:
			return Response{Status: 201, Body: []byte(`{"id":"123"}`)}, nil
		case 2, 3:
			if reports != 1 {
				t.Fatalf("reports=%d", reports)
			}
			return Response{Status: 200, Body: []byte(`{"status":"PENDING"}`)}, nil // Missing capture must not be evaluated.
		case 4:
			return Response{Status: 200, Body: []byte(`{"status":"READY","ready":"done"}`)}, nil
		default:
			if reports != 2 || req.URL != "http://example.test/done" {
				t.Fatalf("reports=%d req=%+v", reports, req)
			}
			return Response{Status: 200}, nil
		}
	}), func(EntryResult) error { reports++; return nil })
	result := r.Run(context.Background(), scenario)
	if result.Err != nil || result.Diverged || reports != 3 || result.Summary != (Summary{3, 3, 3, 3, 0, 0}) || result.Attempts != (AttemptSummary{5, 5, 5}) {
		t.Fatalf("result=%+v reports=%d", result, reports)
	}
	if !reflect.DeepEqual(requests[1], requests[2]) || !reflect.DeepEqual(requests[1], requests[3]) || requests[1].Method != "POST" || string(requests[1].Body) != `{"id":"123"}` {
		t.Fatalf("requests=%+v", requests)
	}
	// A fresh run must not inherit the previous capture or budget.
	requests = nil
	reports = 0
	again := r.Run(context.Background(), scenario)
	if again.Attempts.Started != 5 || again.Err != nil || again.Diverged {
		t.Fatalf("fresh session=%+v", again)
	}
}

func TestPollingCaptureAndPreparationErrors(t *testing.T) {
	for _, suffix := range []string{"[Captures]\nmissing: header \"X-Missing\"\n", "[Captures]\nvalue: jsonpath \"$.missing\"\n"} {
		t.Run(suffix, func(t *testing.T) {
			calls := 0
			r := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(`{}`)}, nil
			}), nil)
			got := r.Run(context.Background(), pollingScenario(t, "GET http://example.test/job\n[Options]\nretry: 3\nretry-interval: 0\nHTTP 200\n"+suffix))
			if calls != 1 || got.Err == nil || got.Diverged || got.Entries[0].Outcome != CaptureFailed {
				t.Fatalf("result=%+v calls=%d", got, calls)
			}
		})
	}
	for _, body := range []string{"file,missing;", `{"value":"{{missing}}"}`} {
		t.Run(body, func(t *testing.T) {
			r := New(executorFunc(func(context.Context, Request) (Response, error) { t.Fatal("unexpected send"); return Response{}, nil }), nil)
			got := r.Run(context.Background(), pollingScenario(t, fmt.Sprintf("POST http://example.test/job\n[Options]\nretry: 3\nretry-interval: 0\n%s\nHTTP 200\n", body)))
			if got.Err == nil || got.Attempts.Started != 0 || got.Summary.Started != 0 || got.Entries[0].Attempts != 0 {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}

func TestPollingDiscardedResponseProtection(t *testing.T) {
	secret := "discarded-session-private"
	renderer, _ := template.New(nil, nil)
	scenario := pollingScenario(t, "GET http://example.test/job\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200\n[Asserts]\njsonpath \"$.status\" == \"READY\"\n")
	calls := 0
	r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
		calls++
		if calls == 1 {
			return Response{Status: 503, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: secret}}}, nil
		}
		return Response{Status: 200, Body: []byte(`{"status":"` + secret + `"}`)}, nil
	}), renderer, func(entry EntryResult) error {
		if strings.Contains(fmt.Sprintf("%+v", entry.JSONFailure), secret) {
			t.Fatal("callback exposed secret")
		}
		return nil
	})
	got := r.Run(context.Background(), scenario)
	if !got.Diverged || calls != 2 || strings.Contains(fmt.Sprintf("%+v", got.Entries[0].JSONFailure), secret) {
		t.Fatalf("result=%+v", got)
	}
}
