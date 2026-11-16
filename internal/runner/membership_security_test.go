package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

func TestMembershipSessionsAndReportsStayIndependent(t *testing.T) {
	for _, line := range []string{`jsonpath "$.session_private" contains "session_private"`, `jsonpath "$.session_private" not contains "session_private"`, `jsonpath "$.session_private" not exists`} {
		s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n"+line)
		before, _ := json.Marshal(s)
		secret, _ := template.New([]string{"token=session_private"}, []string{"token"})
		plain, _ := template.New(nil, nil)
		for _, renderer := range []Renderer{secret, plain, secret} {
			for _, body := range []string{`{"session_private":[]}`, `{"session_private":["session_private"]}`} {
				check := func(entry EntryResult) error {
					t.Helper()
					for _, a := range entry.Entry.Assertions {
						if a.JSON != nil && (a.JSON.Expected != nil || a.JSON.Compiled != nil || a.JSON.Pattern != "") {
							t.Fatal("raw operand retained")
						}
					}
					raw, _ := json.Marshal(entry)
					if renderer == secret && strings.Contains(string(raw), "session_private") {
						t.Fatalf("leaked: %s", raw)
					}
					if entry.JSONFailure != nil && entry.JSONFailure.Actual != "tipo array" && entry.JSONFailure.Actual != "presente" {
						t.Fatalf("unsafe actual: %+v", entry.JSONFailure)
					}
					return nil
				}
				r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
					return Response{Status: 200, Body: []byte(body)}, nil
				}), renderer, check)
				for range 2 {
					got := r.Run(context.Background(), s)
					if got.Err != nil || len(got.Entries) != 1 {
						t.Fatalf("%+v", got)
					}
					_ = check(got.Entries[0])
					if failure := got.Entries[0].JSONFailure; failure != nil && renderer == plain && !strings.Contains(failure.Query, "session_private") {
						t.Fatal("secret state leaked into plain session")
					}
				}
			}
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("scenario mutated")
		}
	}
}

func TestMembershipDerivedSecretsAndPendingMetadata(t *testing.T) {
	for _, pending := range []bool{false, true} {
		renderer, _ := template.New([]string{"token=session_private"}, []string{"token", "future"})
		source := "GET http://example.test/login\nAuthorization: Bearer {{token}}\nGET http://example.test/second\nHTTP *\n[Asserts]\njsonpath \"$.session_private\" contains \"Bearer session_private\""
		if pending {
			source += "\n[Captures]\nfuture: jsonpath \"$.missing.bad\""
		}
		source += "\nGET http://example.test/never"
		s := parseCaptureScenario(t, source)
		// Filenames and queries are presentation metadata too.
		s.Entries[1].Assertions[0].JSON.Position.File = "session_private.hurl"
		calls := 0
		result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(`{"session_private":["unrelated-private"]}`)}, nil
		}), renderer, func(entry EntryResult) error {
			raw, _ := json.Marshal(entry)
			for _, secret := range []string{"session_private", "unrelated-private"} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("leaked %s", raw)
				}
			}
			return nil
		}).Run(context.Background(), s)
		if calls != 2 || result.Err != nil || !result.Diverged {
			t.Fatalf("%+v calls=%d", result, calls)
		}
		failure := result.Entries[1].JSONFailure
		if failure.Expected != `"[oculto]"` && failure.Expected != "[oculto]" {
			t.Fatalf("%+v", failure)
		}
	}
}

func TestMembershipCallbackFailurePreservesEarlierResults(t *testing.T) {
	for _, body := range []string{`[]`, `["forbidden"]`, `invalid`} {
		s := parseCaptureScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/assert\nHTTP *\n[Asserts]\njsonpath \"$\" not contains \"forbidden\"\nGET http://example.test/never")
		cause := errors.New("report failure")
		calls, reports := 0, 0
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
		if calls != 2 || reports != 2 || len(got.Entries) != 2 || got.Entries[0].Outcome != Passed || !errors.Is(got.ReportErr, cause) || got.Summary != (Summary{Total: 3, Started: 2, Completed: 2, Validated: 1, WithoutValidation: 1, NotStarted: 1}) {
			t.Fatalf("%+v calls=%d reports=%d", got, calls, reports)
		}
	}
}
