package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-273–277/279: the existing MatchString semantics now apply to the first header.
func TestHeaderMatchesSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, pattern string
		values        []string
		pass          bool
	}{
		{"partial", `"Pizza"`, []string{"Mini Pizza"}, true},
		{"anchors", `"^Pizza$"`, []string{"Mini Pizza"}, false},
		{"case sensitive", `"pizza"`, []string{"Pizza"}, false},
		{"case option", `"(?i)^pizza$"`, []string{"Pizza"}, true},
		{"unicode", `"^\\p{L}+$"`, []string{"雪é"}, true},
		{"no normalization", `"é"`, []string{"é"}, false},
		{"dotall option", `"(?s)^a.b$"`, []string{"a\nb"}, true},
		{"multiline option", `"(?m)^b$"`, []string{"a\nb"}, true},
		{"JSON escapes", `"^a\\s\"b\\\\c/$"`, []string{"a \"b\\c/"}, true},
		{"groups classes repetition", `"^(ab|cd)[0-9]{2}$"`, []string{"cd12"}, true},
		{"empty pattern value", `""`, []string{"x"}, true},
		{"empty pattern empty", `""`, []string{""}, true},
		{"empty pattern absent", `""`, nil, false},
		{"empty anchor", `"^$"`, []string{""}, true},
		{"empty anchor value", `"^$"`, []string{"x"}, false},
		{"empty anchor absent", `"^$"`, nil, false},
		{"first wins", `"^ok$"`, []string{"ok", "bad"}, true},
		{"second cannot fix", `"^ok$"`, []string{"bad", "ok"}, false},
		{"first empty", `"^$"`, []string{"", "bad"}, true},
		{"second cannot fix empty", `"^ok$"`, []string{"", "ok"}, false},
		{"comma preserved", `"^first,whole$"`, []string{"first,whole", "other"}, true},
		{"literal placeholder", `"^{{name}}$"`, []string{"{{name}}"}, true},
		{"placeholder not resolved", `"^{{name}}$"`, []string{"resolved"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, assignments := range [][]string{nil, {"name=resolved"}} {
				c, err := template.New(assignments, nil)
				if err != nil {
					t.Fatal(err)
				}
				s := parseCaptureScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\nheader \"x-TEST\" matches "+tc.pattern+"\nGET http://example.test/next")
				headers := make([]query.HeaderValue, len(tc.values))
				for i, v := range tc.values {
					headers[i] = query.HeaderValue{Name: "X-Test", Value: v}
				}
				calls := 0
				got := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
					calls++
					if req.RetainBody {
						t.Fatal("header matches requested body retention")
					}
					return Response{Status: 200, Headers: headers}, nil
				}), c, nil).Run(context.Background(), s)
				wantCalls, wantOutcome := 1, HeaderMismatch
				if tc.pass {
					wantCalls, wantOutcome = 2, Passed
				}
				if got.Err != nil || calls != wantCalls || got.Entries[0].Outcome != wantOutcome {
					t.Fatalf("calls=%d result=%+v", calls, got)
				}
				if !tc.pass && got.Entries[0].HeaderFailure.Present != (len(tc.values) > 0) {
					t.Fatalf("presence=%+v", got.Entries[0].HeaderFailure)
				}
			}
		})
	}
}

// GO-280: status, source-ordered assertions, then atomic captures.
func TestHeaderMatchesOrderAndCaptures(t *testing.T) {
	for _, tc := range []struct {
		name, expectation, assertions, body string
		status                              int
		value                               string
		outcome                             Outcome
		calls                               int
	}{
		{"status first", "HTTP 200", `header "X" matches "^ok$"`, "invalid", 503, "bad", Mismatch, 1},
		{"header first", "HTTP 200", "header \"X\" matches \"^ok$\"\njsonpath \"$\" exists", "\"captured\"", 200, "bad", HeaderMismatch, 1},
		{"JSON first", "HTTP 200", "jsonpath \"$\" exists\nheader \"X\" matches \"^ok$\"", "invalid", 200, "bad", CaptureFailed, 1},
		{"later header", "HTTP 200", "header \"X\" matches \"^ok$\"\nheader \"Missing\" exists", `"captured"`, 200, "ok", HeaderMismatch, 1},
		{"capture fails", "HTTP 200", `header "X" matches "^ok$"`, "invalid", 200, "ok", CaptureFailed, 1},
		{"publish", "HTTP 200", "header \"X\" matches \"^ok$\"\njsonpath \"$\" == \"captured\"", `"captured"`, 200, "ok", Passed, 2},
		{"wildcard", "HTTP *", `header "X" matches "^ok$"`, `"captured"`, 503, "ok", Wildcard, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := parseCaptureScenario(t, "GET http://example.test/first\n"+tc.expectation+"\n[Captures]\nid: jsonpath \"$\"\n[Asserts]\n"+tc.assertions+"\nGET http://example.test/{{id}}")
			calls := 0
			got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if calls == 2 && req.URL != "http://example.test/captured" {
					t.Fatalf("request=%+v", req)
				}
				return Response{Status: tc.status, Headers: []query.HeaderValue{{Name: "X", Value: tc.value}}, Body: []byte(tc.body)}, nil
			}), nil).Run(context.Background(), s)
			if calls != tc.calls || got.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d result=%+v", calls, got)
			}
		})
	}
}

func TestHeaderMatchesSensitiveFailure(t *testing.T) {
	for _, name := range []string{"Authorization", "Proxy-Authorization", "Cookie", "sEt-CoOkIe"} {
		s := parseCaptureScenario(t, fmt.Sprintf("GET http://example.test\nHTTP 200\n[Asserts]\nheader %q matches \"expected-private\"", name))
		inspect := func(result EntryResult) error {
			h, failure := result.Entry.Assertions[0].Header, result.HeaderFailure
			if h.Compiled != nil || h.Expected != "[oculto]" || failure.Expected != "[oculto]" || failure.Actual != "[oculto]" || failure.Name != name || failure.Operator != "matches" || failure.Position.Line != 4 {
				t.Fatalf("header=%+v failure=%+v", h, failure)
			}
			if strings.Contains(fmt.Sprintf("%+v", result), "private") {
				t.Fatalf("unsafe result=%+v", result)
			}
			return nil
		}
		got := New(executorFunc(func(context.Context, Request) (Response, error) {
			return Response{Status: 200, Headers: []query.HeaderValue{{Name: name, Value: "actual-private"}}}, nil
		}), inspect).Run(context.Background(), s)
		if !got.Diverged {
			t.Fatalf("result=%+v", got)
		}
		_ = inspect(got.Entries[0])
		if s.Entries[0].Assertions[0].Header.Compiled == nil {
			t.Fatal("scenario mutated")
		}
	}
}
