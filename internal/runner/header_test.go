package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func TestHeaderPrecedenceAndPublication(t *testing.T) {
	for _, tc := range []struct {
		name, expectation, body string
		status                  int
		headers                 []query.HeaderValue
		outcome                 Outcome
		calls, validated        int
	}{
		{"success", "HTTP 200", `"captured"`, 200, []query.HeaderValue{{Name: "X-Test", Value: "ok"}}, Passed, 2, 1},
		{"wildcard success", "HTTP *", `"captured"`, 503, []query.HeaderValue{{Name: "X-Test", Value: "ok"}}, Wildcard, 2, 0},
		{"status first", "HTTP 200", `invalid`, 503, nil, Mismatch, 1, 0},
		{"header before capture", "HTTP 200", `invalid`, 200, nil, HeaderMismatch, 1, 1},
		{"wildcard header failure", "HTTP *", `invalid`, 404, nil, HeaderMismatch, 1, 0},
		{"capture after header", "HTTP 200", `invalid`, 200, []query.HeaderValue{{Name: "X-Test", Value: "ok"}}, CaptureFailed, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "GET http://example.test/first\n" + tc.expectation + "\n[Captures]\ntoken: jsonpath \"$\"\n[Asserts]\nheader \"x-test\" == \"ok\"\nGET http://example.test/{{token}}"
			s := parseCaptureScenario(t, source)
			calls, reports := 0, 0
			got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				if reports != calls {
					t.Fatal("report not synchronous")
				}
				calls++
				if calls == 2 && req.URL != "http://example.test/captured" {
					t.Fatalf("req=%+v", req)
				}
				return Response{Status: tc.status, Body: []byte(tc.body), Headers: tc.headers}, nil
			}), func(EntryResult) error { reports++; return nil }).Run(context.Background(), s)
			if calls != tc.calls || got.Entries[0].Outcome != tc.outcome || got.Summary.Validated != tc.validated {
				t.Fatalf("calls=%d got=%+v", calls, got)
			}
			if tc.outcome == HeaderMismatch && (got.Err != nil || !got.Diverged || got.Entries[0].HeaderFailure.Present) {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}

func TestHeaderReporterSanitized(t *testing.T) {
	for _, name := range []string{"aUtHoRiZaTiOn", "PROXY-authorization", "Cookie", "SET-cookie"} {
		t.Run(name, func(t *testing.T) {
			source := fmt.Sprintf("GET http://example.test/actual-private\nHTTP *\n[Asserts]\nheader %q == \"expected-private\"\nGET http://example.test/last", name)
			s := parseCaptureScenario(t, source)
			r := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				if req.RetainBody {
					t.Fatal("header assertion retained body")
				}
				return Response{Status: 200, Headers: []query.HeaderValue{{Name: name, Value: "actual-private"}}}, nil
			}), func(result EntryResult) error {
				encoded := fmt.Sprintf("%+v %+v %+v", result, result.Entry, result.HeaderFailure)
				if strings.Contains(encoded, "actual-private") || strings.Contains(encoded, "expected-private") {
					t.Fatalf("raw callback: %s", encoded)
				}
				return nil
			})
			got := r.Run(context.Background(), s)
			failure := got.Entries[0].HeaderFailure
			if !got.Diverged || failure.Expected != "[oculto]" || failure.Actual != "[oculto]" || got.Entries[0].DisplayURL != "http://example.test/[oculto]" || s.Entries[0].Assertions[0].Header.Expected != "expected-private" {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}

func TestCommonHeaderKnownSecretsAndLiteral(t *testing.T) {
	for _, assignment := range [][]string{nil, {"name=resolved"}} {
		c, _ := template.New(assignment, []string{"token"})
		s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\nheader \"X-Literal\" == \"{{name}}\"\nheader \"X-Common\" == \"expected-private\"")
		got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			return Response{Status: 200, Headers: []query.HeaderValue{{Name: "X-Literal", Value: "{{name}}"}, {Name: "Authorization", Value: "actual-private"}, {Name: "X-Common", Value: "actual-private"}}}, nil
		}), c, nil).Run(context.Background(), s)
		if !got.Diverged || got.Entries[0].HeaderFailure.Actual != "[oculto]" || got.Entries[0].HeaderFailure.Name != "X-Common" {
			t.Fatalf("got=%+v", got)
		}
	}
	// Known initial values are protected even on ordinary headers.
	c, _ := template.New([]string{"token=known-private"}, []string{"token"})
	s := parser.Scenario{Entries: []parser.Entry{{Assertions: []parser.Assertion{{Header: &parser.HeaderAssertion{Name: "X", Expected: "known-private", Operator: "=="}}}}}}
	got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) { return Response{Status: 200}, nil }), c, nil).Run(context.Background(), s)
	if got.Entries[0].HeaderFailure.Expected != "[oculto]" {
		t.Fatalf("got=%+v", got)
	}
}
