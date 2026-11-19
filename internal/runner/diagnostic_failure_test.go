package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/template"
)

func TestDiagnosticBeforeSendAndTransportFailureNeverEchoTemplate(t *testing.T) {
	for _, preparation := range []bool{false, true} {
		source := "POST http://example.test\n{\"token\":\"not-sent-private\"}\nHTTP 200"
		if preparation {
			source = "POST http://example.test/{{missing}}\n{\"token\":\"not-sent-private\"}\nHTTP 200"
		}
		s := pollingScenario(t, source)
		calls := 0
		r := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{}, errors.New("raw-private-transport")
		}), func(entry EntryResult) error {
			if entry.Bodies == nil || entry.Bodies.Request.State != BodyUnavailable || entry.Bodies.Response.State != BodyUnavailable || entry.Bodies.Request.Text != "" || entry.Bodies.Response.Text != "" {
				t.Fatalf("preview=%+v", entry.Bodies)
			}
			if entry.Diagnostic == nil || strings.Contains(entry.Diagnostic.Detail, "private") {
				t.Fatalf("unsafe diagnostic %+v", entry.Diagnostic)
			}
			return nil
		})
		result := r.RunWithOptions(context.Background(), s, Options{DiagnosticBodies: true})
		want := 1
		if preparation {
			want = 0
		}
		if result.Err == nil || len(result.Entries) != 1 || calls != want {
			t.Fatalf("calls=%d result=%+v", calls, result)
		}
	}
}

func TestDiagnosticPollingProtectsLastBytesAtFinalization(t *testing.T) {
	c, _ := template.New(nil, nil)
	session := c.NewSession()
	s := pollingScenario(t, "POST http://example.test\n{\"value\":\"late-known-private\"}\n[Options]\nretry: 1\nHTTP 200")
	r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
		return diagnosticResponse(req, 503, `{"value":"late-known-private"}`), nil
	}), observedHeaderSession{session}, nil)
	r.wait = func(context.Context, time.Duration) error {
		session.ProtectValue("late-known-private")
		return context.Canceled
	}
	got := r.RunWithOptions(context.Background(), s, Options{DiagnosticBodies: true})
	if got.Err == nil || len(got.Entries) != 1 || got.Entries[0].Bodies == nil {
		t.Fatalf("result=%+v", got)
	}
	previews := got.Entries[0].Bodies
	if strings.Contains(previews.Request.Text+previews.Response.Text, "late-known-private") || !strings.Contains(previews.Request.Text, "[oculto]") || !strings.Contains(previews.Response.Text, "[oculto]") {
		t.Fatalf("unsafe last previews %+v", previews)
	}
}
