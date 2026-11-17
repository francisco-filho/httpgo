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

func TestBodyReportsNeverRetainOperationalData(t *testing.T) {
	for _, mode := range []string{"pending", "known", "derived", "charset", "success"} {
		t.Run(mode, func(t *testing.T) {
			renderer, err := template.New([]string{"secret=expected-private"}, []string{"secret", "token"})
			if err != nil {
				t.Fatal(err)
			}
			source := "GET http://example.test\nHTTP 200\n[Asserts]\nbody contains \"expected-private\"\n[Captures]\ntoken: jsonpath \"$\"\nGET http://example.test/next"
			if mode != "pending" {
				source = strings.Replace(source, "\n[Captures]\ntoken: jsonpath \"$\"", "", 1)
			}
			if mode == "derived" {
				source = "GET http://example.test/login\nAuthorization: Bearer {{secret}}\n" + strings.Replace(source, "expected-private", "Bearer expected-private", 1)
			}
			s := parseCaptureScenario(t, source)
			before, _ := json.Marshal(s)
			calls, reports := 0, 0
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				body := `{"value":"actual-private","unused":"body-private"}`
				if mode == "charset" {
					body = "\xffactual-private body-private"
				}
				if mode == "success" {
					body = "expected-private actual-private body-private"
				}
				return Response{Status: 200, Body: []byte(body)}, nil
			}), renderer, func(entry EntryResult) error {
				reports++
				assertBodyReportSafe(t, entry)
				return nil
			}).Run(context.Background(), s)
			if reports != calls {
				t.Fatalf("reports=%d calls=%d", reports, calls)
			}
			for _, entry := range result.Entries {
				assertBodyReportSafe(t, entry)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("scenario mutated")
			}
			switch mode {
			case "charset":
				var failure *BodyError
				if !errors.As(result.Err, &failure) || result.Diverged {
					t.Fatalf("result=%+v", result)
				}
			case "success":
				if result.Err != nil || result.Diverged || calls != 2 {
					t.Fatalf("result=%+v calls=%d", result, calls)
				}
			default:
				if !result.Diverged || result.Err != nil {
					t.Fatalf("result=%+v", result)
				}
				failure := result.Entries[len(result.Entries)-1].BodyFailure
				if failure == nil || strings.Contains(failure.Expected, "private") {
					t.Fatalf("failure=%+v", failure)
				}
			}
		})
	}
}

func assertBodyReportSafe(t *testing.T, entry EntryResult) {
	t.Helper()
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"expected-private", "actual-private", "body-private"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("report leaks %q: %s", forbidden, data)
		}
	}
	for _, declaration := range entry.Entry.Assertions {
		if declaration.Body != nil && declaration.Body.Expected != "" {
			t.Fatal("raw body literal retained in metadata")
		}
	}
}

func TestBodyScenarioAndSessionsAreIndependent(t *testing.T) {
	s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\nbody contains \"session-private\"")
	before, _ := json.Marshal(s)
	secret, _ := template.New([]string{"token=session-private"}, []string{"token"})
	plain, _ := template.New(nil, nil)
	for _, renderer := range []Renderer{secret, plain, secret} {
		r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			return Response{Status: 200, Body: []byte("different")}, nil
		}), renderer, nil)
		for range 2 {
			result := r.Run(context.Background(), s)
			want := "session-private"
			if renderer == secret {
				want = "[oculto]"
			}
			if result.Err != nil || !result.Diverged || result.Entries[0].BodyFailure.Expected != want {
				t.Fatalf("result=%+v want=%q", result, want)
			}
			if !reflect.DeepEqual(result.Entries[0].Entry.Assertions[0].Body, &parser.BodyAssertion{Operator: s.Entries[0].Assertions[0].Body.Operator, Position: s.Entries[0].Assertions[0].Body.Position}) {
				t.Fatal("unexpected report metadata")
			}
		}
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("scenario mutated")
	}
}

func TestBodyCallbackFailurePreservesResultsAndStops(t *testing.T) {
	for _, body := range []string{"expected", "different", "\xff"} {
		s := parseCaptureScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/body\nHTTP *\n[Asserts]\nbody contains \"expected\"\nGET http://example.test/never")
		calls, reports := 0, 0
		cause := errors.New("report failure")
		result := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(body)}, nil
		}), func(EntryResult) error {
			reports++
			if reports == 2 {
				return cause
			}
			return nil
		}).Run(context.Background(), s)
		if calls != 2 || reports != 2 || len(result.Entries) != 2 || result.Entries[0].Outcome != Passed || !errors.Is(result.ReportErr, cause) || result.Summary.NotStarted != 1 {
			t.Fatalf("result=%+v calls=%d reports=%d", result, calls, reports)
		}
	}
}
