package transport

import (
	"context"
	"errors"
	"fmt"
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

// GO-359: only the final complete response may be selected, not a valid JSON
// prefix or an intermediate redirect body. Channels synchronize cancellation.
func TestFilterFinalResponseAndCancellation(t *testing.T) {
	for _, capture := range []bool{false, true} {
		for _, mode := range []string{"final", "incomplete", "timeout", "cancel"} {
			t.Run(fmt.Sprintf("%s/capture=%v", mode, capture), func(t *testing.T) {
				received, closed := make(chan struct{}), make(chan struct{})
				var hits atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits.Add(1)
					if r.URL.Path == "/start" {
						w.Header().Set("Location", "/final")
						w.WriteHeader(307)
						_, _ = io.WriteString(w, `[{"key":"selected","id":"wrong"}]`)
						return
					}
					if r.URL.Path == "/next" {
						body, _ := io.ReadAll(r.Body)
						if capture && string(body) != `["final","second"]` {
							t.Errorf("capture=%s", body)
						}
						return
					}
					if mode != "final" {
						w.Header().Set("Content-Length", "200")
					}
					_, _ = io.WriteString(w, `[{"key":"selected","id":"final"},{"key":"selected","id":"second"}]`)
					if mode == "timeout" || mode == "cancel" {
						w.(http.Flusher).Flush()
						close(received)
						<-r.Context().Done()
						close(closed)
					}
				}))
				defer server.Close()
				lines := "[Asserts]\njsonpath \"$[?(@.key=='selected')].id\" == [\"final\",\"second\"]"
				next := "GET " + server.URL + "/next"
				if capture {
					lines = "[Captures]\nx: jsonpath \"$[?(@.key=='selected')].id\""
					next = "POST " + server.URL + "/next\n{{x}}"
				}
				s, err := parser.Parse("filter.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP *\n"+lines+"\n"+next))
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
						t.Fatal("no final headers")
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
						t.Fatalf("result=%+v hits=%d", got, hits.Load())
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
					t.Fatalf("result=%+v hits=%d", got, hits.Load())
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
}

// GO-359–360: report callbacks observe closed resources even when Read/Close
// fails after returning an otherwise valid document.
func TestFilterReadCloseBeforeReport(t *testing.T) {
	for _, mode := range []string{"success", "read", "close"} {
		t.Run(mode, func(t *testing.T) {
			reader := strings.NewReader(`[{"x":1,"id":1},{"x":1,"id":2}]`)
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
			s, err := parser.Parse("filter.hurl", strings.NewReader("GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$[?(@.x==1)].id\" count == 2\nGET http://example.test/never"))
			if err != nil {
				t.Fatal(err)
			}
			got := runner.New(regexCaptureExecutor{executor: executor, t: t, retain: true}, func(runner.EntryResult) error {
				if !body.closed || reader.Len() != 0 {
					t.Error("report before close")
				}
				return errors.New("stop report")
			}).Run(context.Background(), s)
			if calls != 1 || got.ReportErr == nil || got.Summary.NotStarted != 1 {
				t.Fatalf("result=%+v", got)
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
