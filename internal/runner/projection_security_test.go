package runner

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

func TestProjectionReportsAndSessionsIndependent(t *testing.T) {
	s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$.private_query[*]\" count == 0\nGET http://example.test/never")
	before, _ := json.Marshal(s)
	secret, _ := template.New([]string{"token=private_query"}, []string{"token"})
	plain, _ := template.New(nil, nil)
	for _, renderer := range []Renderer{secret, plain, secret} {
		check := func(e EntryResult) error {
			a := e.Entry.Assertions[0].JSON
			if !reflect.DeepEqual(a.Path, parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$\" exists").Entries[0].Assertions[0].JSON.Path) || a.Expected != nil {
				t.Fatal("operational data retained")
			}
			raw, _ := json.Marshal(e)
			if strings.Contains(string(raw), "collection-private") || renderer == secret && strings.Contains(string(raw), "private_query") {
				t.Fatalf("leaked: %s", raw)
			}
			return nil
		}
		r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			return Response{Status: 200, Body: []byte(`{"private_query":["collection-private","second"]}`)}, nil
		}), renderer, check)
		for range 2 {
			got := r.Run(context.Background(), s)
			if got.Err != nil || !got.Diverged {
				t.Fatalf("%+v", got)
			}
			_ = check(got.Entries[0])
		}
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("scenario mutated")
	}
}

func TestProjectionCallbackFailurePreservesCounts(t *testing.T) {
	for _, body := range []string{`[]`, `["forbidden","other"]`, `[null,{"x":1}]`} {
		s := parseCaptureScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/assert\nHTTP *\n[Asserts]\njsonpath \"$[*]\" not contains \"forbidden\"\nGET http://example.test/never")
		calls, reports := 0, 0
		cause := errors.New("report failure")
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
		if calls != 2 || reports != 2 || !errors.Is(got.ReportErr, cause) || got.Entries[0].Outcome != Passed || got.Summary != (Summary{Total: 3, Started: 2, Completed: 2, Validated: 1, WithoutValidation: 1, NotStarted: 1}) {
			t.Fatalf("%+v", got)
		}
	}
}

func TestProjectionPublishedValuesDoNotCrossRuns(t *testing.T) {
	c, _ := template.New(nil, []string{"x"})
	s := parseCaptureScenario(t, "GET http://example.test\n[Captures]\nx: jsonpath \"$[*]\"\nPOST http://example.test/echo\n{{x}}")
	calls := 0
	r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
		calls++
		if req.Method == "POST" && string(req.Body) != `[9007199254740993,"private"]` {
			t.Fatalf("%s", req.Body)
		}
		return Response{Status: 200, Body: []byte(`[9007199254740993,"private"]`)}, nil
	}), c, nil)
	for range 2 {
		if got := r.Run(context.Background(), s); got.Err != nil {
			t.Fatal(got.Err)
		}
	}
	got := r.Run(context.Background(), parseCaptureScenario(t, "GET http://example.test/{{x}}"))
	if got.Err == nil || got.Summary.Started != 0 || calls != 4 {
		t.Fatalf("leaked capture: %+v calls=%d", got, calls)
	}
}
