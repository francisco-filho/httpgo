package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-386,390,392: query errors remain category 2, but only after a valid expected.
func TestRenderAssertsQueryErrorPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, line, body, value, detail string
		headers                         []query.HeaderValue
		outcome                         Outcome
	}{
		{"JSON syntax", `jsonpath "$" == {{v}}`, `bad`, `null`, "resposta não contém", nil, JSONFailed},
		{"JSON traversal", `jsonpath "$.x.y" not contains {{v}}`, `{"x":1}`, `null`, "percurso", nil, JSONFailed},
		{"body charset", `body not contains "{{v}}"`, `body`, `"x"`, "charset", []query.HeaderValue{{Name: "Content-Type", Value: "text/plain; charset=unsupported"}}, BodyFailed},
		{"expected before charset", `body not contains "{{missing}}"`, `body`, `null`, "variável ausente", []query.HeaderValue{{Name: "Content-Type", Value: "text/plain; charset=unsupported"}}, BodyFailed},
		{"regex before JSON", `jsonpath "$" matches "{{v}}"`, `bad`, `"["`, "padrão matches", nil, JSONFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := dynamicScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\n"+tc.line+"\n[Captures]\nsecret: jsonpath \"$.invalid\"\nGET http://example.test/never")
			c, _ := template.New([]string{"v=" + tc.value}, []string{"secret"})
			calls := 0
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(tc.body), Headers: tc.headers}, nil
			}), c, nil).Run(context.Background(), s)
			if calls != 1 || result.Diverged || result.Err == nil || result.Entries[0].Outcome != tc.outcome || !strings.Contains(result.Err.Error(), tc.detail) {
				t.Fatalf("calls=%d result=%+v err=%v", calls, result, result.Err)
			}
		})
	}
}

// GO-390: pending secret captures do not resolve early, even for dynamic assertions.
func TestRenderAssertsPendingSecretsAndScenarioReuse(t *testing.T) {
	s := dynamicScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\nheader \"X\" == \"{{v}}\"\n[Captures]\nsecret: jsonpath \"$.missing.bad\"\nGET http://example.test/never")
	for _, expected := range []string{"first-private", "second-private"} {
		c, _ := template.New([]string{"v=" + expected}, []string{"secret"})
		calls := 0
		result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Headers: []query.HeaderValue{{Name: "X", Value: "actual-private"}}}, nil
		}), c, func(e EntryResult) error {
			if e.HeaderFailure.Expected != "[oculto]" || e.HeaderFailure.Actual != "[oculto]" {
				t.Fatalf("callback: %+v", e.HeaderFailure)
			}
			return nil
		}).Run(context.Background(), s)
		if calls != 1 || !result.Diverged || result.Err != nil {
			t.Fatalf("result=%+v", result)
		}
	}
	if s.Entries[0].Assertions[0].Header.ExpectedTemplate.Source != "{{v}}" {
		t.Fatal("scenario mutated")
	}
}

// GO-394: no expected preparation from partial/error responses, no retry on category 2.
func TestRenderAssertsTransportAndReportFailures(t *testing.T) {
	for _, mode := range []string{"transport", "cancel", "expected", "report"} {
		t.Run(mode, func(t *testing.T) {
			s := dynamicScenario(t, "GET http://example.test\n[Options]\nretry: 2\nretry-interval: 0\nHTTP 200\n[Asserts]\njsonpath \"$\" == {{missing}}\nGET http://example.test/never")
			cause := errors.New("private I/O cause")
			calls, reports := 0, 0
			result := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				if mode == "transport" {
					return Response{Status: 200, Body: []byte(`null`)}, cause
				}
				if mode == "cancel" {
					return Response{}, context.Canceled
				}
				return Response{Status: 200, Body: []byte(`null`)}, nil
			}), func(e EntryResult) error {
				reports++
				if mode == "report" {
					return cause
				}
				return nil
			}).Run(context.Background(), s)
			if calls != 1 || reports != 1 || result.Diverged || result.Err == nil || result.Summary.NotStarted != 1 {
				t.Fatalf("result=%+v calls=%d", result, calls)
			}
			if mode == "report" && !errors.Is(result.ReportErr, cause) {
				t.Fatalf("report err=%v", result.ReportErr)
			}
			if mode == "transport" && (!errors.Is(result.Err, cause) || result.Summary.Completed != 0 || result.Entries[0].JSONError != nil) {
				t.Fatalf("prepared from partial data: %+v", result)
			}
			if mode == "expected" && (result.Attempts.Started != 1 || !strings.Contains(result.Err.Error(), "variável ausente")) {
				t.Fatal(fmt.Sprint(result))
			}
		})
	}
}
