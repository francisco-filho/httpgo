package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-281/286: observe both callbacks and collected structures, not just streams.
func TestHeaderMatchesMetadataAndIsolation(t *testing.T) {
	for _, mode := range []string{"success", "mismatch", "status", "execution", "report"} {
		t.Run(mode, func(t *testing.T) {
			s := parseCaptureScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\nheader \"Set-Cookie\" matches \"^session=expected-private$\"")
			original := s.Entries[0].Assertions[0].Header
			compiled := original.Compiled
			inspect := func(e EntryResult) {
				h := e.Entry.Assertions[0].Header
				if h == original || h.Compiled != nil || h.Expected != "[oculto]" || e.Entry.Headers != nil {
					t.Fatalf("metadata=%+v", h)
				}
				raw, err := json.Marshal(e)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(raw), "private") || strings.Contains(string(raw), "session=") {
					t.Fatalf("unsafe metadata: %s", raw)
				}
			}
			r := New(executorFunc(func(context.Context, Request) (Response, error) {
				status, value := 200, "session=expected-private"
				if mode == "status" {
					status = 503
				}
				if mode == "mismatch" {
					value = "session=actual-private"
				}
				var err error
				if mode == "execution" {
					err = errors.New("controlled transport failure")
				}
				return Response{Status: status, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: value}}}, err
			}), func(e EntryResult) error {
				inspect(e)
				if mode == "report" {
					return errors.New("controlled write failure")
				}
				return nil
			})
			for range 2 {
				got := r.Run(context.Background(), s)
				if len(got.Entries) != 1 {
					t.Fatalf("result=%+v", got)
				}
				inspect(got.Entries[0])
				if original.Compiled != compiled || original.Expected != "^session=expected-private$" || !compiled.MatchString("session=expected-private") {
					t.Fatal("scenario mutated")
				}
				if (got.ReportErr != nil) != (mode == "report") || (got.Err != nil) != (mode == "execution") || got.Diverged != (mode == "mismatch" || mode == "status") {
					t.Fatalf("result=%+v", got)
				}
			}
		})
	}
	// Protection learned from one response must not contaminate another session.
	c, err := template.New(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := parseCaptureScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\nheader \"X\" matches \"expected-private\"")
	calls := 0
	r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
		calls++
		headers := []query.HeaderValue{{Name: "X", Value: "actual-private"}}
		if calls == 1 {
			headers = append(headers, query.HeaderValue{Name: "Set-Cookie", Value: "expected-private"}, query.HeaderValue{Name: "Cookie", Value: "actual-private"})
		}
		return Response{Status: 200, Headers: headers}, nil
	}), c, nil)
	for i := range 2 {
		got := r.Run(context.Background(), s)
		f := got.Entries[0].HeaderFailure
		expected, actual := "expected-private", "actual-private"
		if i == 0 {
			expected, actual = "[oculto]", "[oculto]"
		}
		if f.Expected != expected || f.Actual != actual || got.Entries[0].Entry.Assertions[0].Header.Compiled != nil {
			t.Fatalf("run=%d failure=%+v", i, f)
		}
	}
}

func TestHeaderMatchesKnownDerivedSecrets(t *testing.T) {
	c, err := template.New([]string{`token={"value":"known-private"}`}, []string{"token"})
	if err != nil {
		t.Fatal(err)
	}
	s := parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\nheader \"X\" matches \"^known-private$\"")
	got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
		return Response{Status: 200, Headers: []query.HeaderValue{{Name: "X", Value: "prefix-known-private"}}}, nil
	}), c, func(e EntryResult) error {
		raw, _ := json.Marshal(e)
		if strings.Contains(string(raw), "known-private") || e.Entry.Assertions[0].Header.Compiled != nil {
			t.Fatalf("metadata=%s", raw)
		}
		return nil
	}).Run(context.Background(), s)
	f := got.Entries[0].HeaderFailure
	if !got.Diverged || f.Expected != "^[oculto]$" || f.Actual != "prefix-[oculto]" {
		t.Fatalf("failure=%+v", f)
	}
}

func TestHeaderMatchesReportFailurePreservesCounts(t *testing.T) {
	for _, mode := range []string{"pass", "mismatch", "execution"} {
		s := parseCaptureScenario(t, "GET http://example.test/prior\nHTTP 200\nGET http://example.test/check\nHTTP 200\n[Asserts]\nheader \"Set-Cookie\" matches \"expected-private\"\nGET http://example.test/never")
		calls, reports := 0, 0
		cause := errors.New("controlled write failure")
		got := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			value := "expected-private"
			if mode == "mismatch" {
				value = "actual-private"
			}
			var err error
			if calls == 2 && mode == "execution" {
				err = errors.New("controlled execution failure")
			}
			return Response{Status: 200, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: value}}}, err
		}), func(e EntryResult) error {
			reports++
			if reports == 2 {
				if e.Entry.Assertions[0].Header.Compiled != nil {
					t.Fatal("compiled regex in callback")
				}
				return cause
			}
			return nil
		}).Run(context.Background(), s)
		completed := 2
		if mode == "execution" {
			completed = 1
		}
		if calls != 2 || reports != 2 || !errors.Is(got.ReportErr, cause) || got.Summary.Started != 2 || got.Summary.Completed != completed || got.Summary.NotStarted != 1 || len(got.Entries) != 2 {
			t.Fatalf("calls=%d reports=%d result=%+v", calls, reports, got)
		}
	}
}
