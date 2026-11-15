package runner

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

func parseCaptureScenario(t *testing.T, source string) parser.Scenario {
	t.Helper()
	s, err := parser.Parse("capture.hurl", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCaptureSequenceStringsAndNewRun(t *testing.T) {
	for _, value := range []string{"123", "null", "true", "", `{{literal}}`, "quotes\"\\\n雪"} {
		scenario := parseCaptureScenario(t, "POST http://example.test/login\nHTTP 200\n[Captures]\nauth.token: jsonpath \"$.items[0].token\"\nPOST http://example.test/products\nX-Token: {{auth.token}}\n{\"value\":{{auth.token}},\"text\":\"pre{{auth.token}}\"}\nHTTP 201")
		before := parseCaptureScenario(t, "POST http://example.test/login\nHTTP 200\n[Captures]\nauth.token: jsonpath \"$.items[0].token\"\nPOST http://example.test/products\nX-Token: {{auth.token}}\n{\"value\":{{auth.token}},\"text\":\"pre{{auth.token}}\"}\nHTTP 201")
		// Control characters are allowed in JSON, but not in a rendered header.
		if strings.Contains(value, "\n") {
			scenario.Entries[1].Headers = nil
			before.Entries[1].Headers = nil
		}
		c, _ := template.New(nil, []string{"auth.token"})
		calls, reports := 0, 0
		r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
			if calls != reports {
				t.Fatal("send before report")
			}
			calls++
			if calls%2 == 1 {
				if !req.RetainBody {
					t.Fatal("capture body not requested")
				}
				raw, _ := json.Marshal(map[string]any{"items": []any{map[string]any{"token": value}}})
				return Response{Status: 200, Body: raw}, nil
			}
			if req.RetainBody {
				t.Fatal("unneeded retention")
			}
			var got map[string]string
			if json.Unmarshal(req.Body, &got) != nil || got["value"] != value || got["text"] != "pre"+value {
				t.Fatalf("body=%s value=%q", req.Body, value)
			}
			if len(req.Headers) > 0 && req.Headers[0].Value != value {
				t.Fatalf("headers=%v", req.Headers)
			}
			return Response{Status: 201}, nil
		}), c, func(EntryResult) error { reports++; return nil })
		for range 2 {
			got := r.Run(context.Background(), scenario)
			if got.Err != nil || got.Summary.Validated != 2 {
				t.Fatalf("%q: %+v", value, got)
			}
		}
		missing := r.Run(context.Background(), parseCaptureScenario(t, "GET http://example.test/next\nX-Token: {{auth.token}}"))
		if missing.Err == nil || calls != 4 || !reflect.DeepEqual(scenario, before) {
			t.Fatalf("isolation/mutation: %+v calls=%d", missing, calls)
		}
	}
}

func TestCaptureFailuresAndStatusPriority(t *testing.T) {
	for _, tc := range []struct {
		name, body, expectation, declarations string
		status                                int
		want                                  Outcome
		validated, unvalidated                int
	}{
		{"invalid JSON", "sensitive", "HTTP 200\n", `token: jsonpath "$"`, 200, CaptureFailed, 1, 0},
		{"missing", `{}`, "HTTP 200\n", `token: jsonpath "$.token"`, 200, CaptureFailed, 1, 0},
		{"null", `null`, "HTTP 200\n", `token: jsonpath "$"`, 200, CaptureFailed, 1, 0},
		{"number", `123`, "", `token: jsonpath "$"`, 200, CaptureFailed, 0, 1},
		{"bool", `false`, "", `token: jsonpath "$"`, 200, CaptureFailed, 0, 1},
		{"array", `[]`, "", `token: jsonpath "$"`, 200, CaptureFailed, 0, 1},
		{"object", `{}`, "", `token: jsonpath "$"`, 200, CaptureFailed, 0, 1},
		{"index", `[]`, "", `token: jsonpath "$[0]"`, 200, CaptureFailed, 0, 1},
		{"mismatch wins", "not JSON", "HTTP 200\n", `token: jsonpath "$"`, 503, Mismatch, 0, 0},
		{"404 allowed", `"token"`, "HTTP 404\n", `token: jsonpath "$"`, 404, Passed, 1, 0},
		{"no expectation", `"token"`, "", `token: jsonpath "$"`, 500, Unvalidated, 0, 1},
		{"batch collision", `"token"`, "", "token: jsonpath \"$\"\ntoken.child: jsonpath \"$\"", 200, CaptureFailed, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := parseCaptureScenario(t, "GET http://example.test/first\n"+tc.expectation+"[Captures]\n"+tc.declarations+"\nGET http://example.test/next")
			calls := 0
			got := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: tc.status, Body: []byte(tc.body)}, nil
			}), nil).Run(context.Background(), s)
			wantCalls := 1
			if tc.want == Passed || tc.want == Unvalidated {
				wantCalls = 2
			}
			if calls != wantCalls || got.Entries[0].Outcome != tc.want || got.Summary.Completed != wantCalls || got.Summary.Validated != tc.validated || got.Summary.WithoutValidation != tc.unvalidated+wantCalls-1 {
				t.Fatalf("%+v calls=%d", got, calls)
			}
			if tc.want == CaptureFailed {
				var err *CaptureError
				if !errors.As(got.Err, &err) || got.ErrorPosition.Line < 3 || strings.Contains(err.Error(), "sensitive") {
					t.Fatalf("%+v", got)
				}
			}
		})
	}
}

func TestCaptureConflictsAndInvalidDependentRequests(t *testing.T) {
	for _, tc := range []struct {
		assign, capture, body, next string
		want                        Outcome
	}{
		{"token=null", "token", `"new"`, "GET http://example.test/next", CaptureFailed},
		{"token=[]", "token.child", `"new"`, "GET http://example.test/next", CaptureFailed},
		{"token.child={}", "token", `"new"`, "GET http://example.test/next", CaptureFailed},
		{"", "token", `"bad space"`, "GET http://example.test/{{token}}", RenderFailed},
		{"", "token", `"bad\r\ninjection"`, "GET http://example.test/next\nX-Token: {{token}}", RenderFailed},
		{"", "token", `"a%2Fb"`, "GET http://example.test/{{token}}", Unvalidated},
	} {
		s := parseCaptureScenario(t, "GET http://example.test/login\n[Captures]\n"+tc.capture+": jsonpath \"$\"\n"+tc.next+"\nGET http://example.test/end")
		var assignments []string
		if tc.assign != "" {
			assignments = []string{tc.assign}
		}
		c, _ := template.New(assignments, nil)
		calls := 0
		got := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
			calls++
			if calls == 2 && strings.Contains(req.URL, "a%2Fb") == false {
				t.Fatal(req.URL)
			}
			return Response{Status: 200, Body: []byte(tc.body)}, nil
		}), c, nil).Run(context.Background(), s)
		if tc.want == Unvalidated {
			if got.Err != nil || calls != 3 {
				t.Fatalf("%+v", got)
			}
		} else if got.Err == nil || got.Entries[len(got.Entries)-1].Outcome != tc.want || calls != 1 {
			t.Fatalf("%+v calls=%d", got, calls)
		}
	}
	// Previous captures cannot be overwritten by a subsequent response.
	s := parseCaptureScenario(t, "GET http://example.test/one\n[Captures]\nauth.token: jsonpath \"$\"\nGET http://example.test/two\n[Captures]\nauth: jsonpath \"$\"\nGET http://example.test/three")
	calls := 0
	got := New(executorFunc(func(context.Context, Request) (Response, error) {
		calls++
		return Response{Status: 200, Body: []byte(`"token"`)}, nil
	}), nil).Run(context.Background(), s)
	if got.Err == nil || calls != 2 || got.Entries[1].Outcome != CaptureFailed {
		t.Fatalf("%+v", got)
	}
}

func TestCaptureFailureReportStopsAndPreservesCounts(t *testing.T) {
	s := parseCaptureScenario(t, "GET http://example.test/login\nHTTP 200\n[Captures]\ntoken: jsonpath \"$\"\nGET http://example.test/next")
	cause := errors.New("writer secret")
	calls := 0
	got := New(executorFunc(func(context.Context, Request) (Response, error) {
		calls++
		return Response{Status: 200, Body: []byte(`null`)}, nil
	}), func(e EntryResult) error {
		if e.Outcome != CaptureFailed || e.Status != 200 {
			t.Fatalf("%+v", e)
		}
		return cause
	}).Run(context.Background(), s)
	if !errors.Is(got.ReportErr, cause) || got.Err == nil || calls != 1 || got.Summary != (Summary{2, 1, 1, 1, 0, 1}) {
		t.Fatalf("%+v", got)
	}
}
