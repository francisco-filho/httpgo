package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

// GO-375: a valid JSON prefix cannot approve isEmpty before full consumption.
func TestJSONTypeFinalAndInterruptedResponse(t *testing.T) {
	for _, mode := range []string{"final", "incomplete", "timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			received, closed := make(chan struct{}), make(chan struct{})
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
					_, _ = io.WriteString(w, `["intermediate"]`)
					return
				}
				if r.URL.Path == "/next" {
					return
				}
				if mode != "final" {
					w.Header().Set("Content-Length", "200")
				}
				_, _ = io.WriteString(w, `[]`)
				if mode == "timeout" || mode == "cancel" {
					w.(http.Flusher).Flush()
					close(received)
					<-r.Context().Done()
					close(closed)
				}
			}))
			defer server.Close()
			s, err := parser.Parse("types.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP *\n[Asserts]\njsonpath \"$\" isEmpty\n[Captures]\nx: jsonpath \"$\"\nGET "+server.URL+"/next"))
			if err != nil {
				t.Fatal(err)
			}
			executor := NewNetHTTPExecutor()
			if mode == "timeout" {
				executor.timeout = 100 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			done := make(chan runner.Result, 1)
			go func() { done <- runner.New(executor, nil).Run(ctx, s) }()
			if mode == "cancel" {
				select {
				case <-received:
					cancel()
				case <-ctx.Done():
					t.Fatal("no final response")
				}
			}
			var got runner.Result
			select {
			case got = <-done:
			case <-time.After(4 * time.Second):
				t.Fatal("runner did not finish")
			}
			if mode == "final" {
				if got.Err != nil || got.Diverged || hits.Load() != 3 || got.Summary.Completed != 2 {
					t.Fatalf("hits=%d %+v", hits.Load(), got)
				}
				return
			}
			kind := Body
			if mode == "timeout" {
				kind = Timeout
			}
			if mode == "cancel" {
				kind = Canceled
			}
			requireKind(t, got.Err, kind)
			if got.Diverged || hits.Load() != 2 || got.Summary.Completed != 0 || got.Entries[0].JSONFailure != nil {
				t.Fatalf("hits=%d %+v", hits.Load(), got)
			}
			if mode == "timeout" || mode == "cancel" {
				select {
				case <-closed:
				case <-time.After(time.Second):
					t.Fatal("response not closed")
				}
			}
		})
	}
}

func TestJSONTypeReadCloseBeforeReport(t *testing.T) {
	for _, mode := range []string{"success", "read", "close"} {
		t.Run(mode, func(t *testing.T) {
			reader := strings.NewReader(`[]`)
			body := &captureBody{Reader: reader}
			if mode == "read" {
				body.Reader = io.MultiReader(reader, errorReader{})
			}
			if mode == "close" {
				body.closeErr = errors.New("private close cause")
			}
			executor := NewNetHTTPExecutor()
			calls := 0
			executor.transport.RegisterProtocol("capture", captureRoundTripper(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: body}, nil
			}))
			s, err := parser.Parse("types.hurl", strings.NewReader("GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$\" isEmpty\nGET http://example.test/never"))
			if err != nil {
				t.Fatal(err)
			}
			got := runner.New(regexCaptureExecutor{executor: executor, t: t, retain: true}, func(runner.EntryResult) error {
				if !body.closed || reader.Len() != 0 {
					t.Error("report before consumption/close")
				}
				return errors.New("stop report")
			}).Run(context.Background(), s)
			if calls != 1 || got.ReportErr == nil || got.Summary.NotStarted != 1 {
				t.Fatalf("%+v", got)
			}
			if mode == "success" {
				if got.Err != nil || got.Summary.Completed != 1 {
					t.Fatal(got)
				}
			} else {
				requireKind(t, got.Err, Body)
				if got.Summary.Completed != 0 {
					t.Fatal(got)
				}
			}
		})
	}
}
