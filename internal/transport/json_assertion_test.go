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

func TestJSONAssertionRequiresSuccessfulReadAndClose(t *testing.T) {
	for _, mode := range []string{"success", "read", "close"} {
		t.Run(mode, func(t *testing.T) {
			body := &captureBody{Reader: strings.NewReader(`{"x":1,"items":[null]}`)}
			if mode == "read" {
				body.Reader = io.MultiReader(body.Reader, errorReader{})
			}
			if mode == "close" {
				body.closeErr = errors.New("private close cause")
			}
			executor := NewNetHTTPExecutor()
			calls := 0
			executor.transport.RegisterProtocol("http", captureRoundTripper(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
			}))
			s, err := parser.Parse("json.hurl", strings.NewReader("GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$.x\" == 1\njsonpath \"$.x\" exists\njsonpath \"$.x\" >= 1\njsonpath \"$.items\" count == 1"))
			if err != nil {
				t.Fatal(err)
			}
			result := runner.New(executor, nil).Run(context.Background(), s)
			if !body.closed || calls != 1 {
				t.Fatalf("closed=%v calls=%d", body.closed, calls)
			}
			if mode == "success" {
				if result.Err != nil || result.Diverged {
					t.Fatalf("result=%+v", result)
				}
			} else {
				requireKind(t, result.Err, Body)
				if result.Summary.Completed != 0 || result.Entries[0].Outcome != runner.Failed || result.Diverged {
					t.Fatalf("result=%+v", result)
				}
			}
		})
	}
}

func TestJSONAssertionFinalBodyDeadlineAndCancellation(t *testing.T) {
	for _, mode := range []string{"success", "timeout", "cancel", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			ready := make(chan struct{})
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(302)
					_, _ = io.WriteString(w, `{"x":"wrong"}`)
					return
				}
				if mode != "success" {
					w.Header().Set("Content-Length", "100")
				}
				_, _ = io.WriteString(w, `{"x":1,"items":[null]}`)
				if mode == "timeout" || mode == "cancel" {
					w.(http.Flusher).Flush()
					close(ready)
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			s, err := parser.Parse("json.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP *\n[Asserts]\njsonpath \"$.x\" == 1\njsonpath \"$.x\" exists\njsonpath \"$.x\" >= 1\njsonpath \"$.items\" count == 1\nGET "+server.URL+"/next"))
			if err != nil {
				t.Fatal(err)
			}
			executor := NewNetHTTPExecutor()
			executor.timeout = 150 * time.Millisecond
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan runner.Result, 1)
			go func() { done <- runner.New(executor, nil).Run(ctx, s) }()
			if mode == "cancel" {
				select {
				case <-ready:
					cancel()
				case <-time.After(3 * time.Second):
					t.Fatal("response not started")
				}
			}
			select {
			case result := <-done:
				if mode == "success" {
					if result.Err != nil || result.Diverged || hits.Load() != 3 {
						t.Fatalf("result=%+v hits=%d", result, hits.Load())
					}
					return
				}
				kind := Timeout
				if mode == "cancel" {
					kind = Canceled
				}
				if mode == "incomplete" {
					kind = Body
				}
				requireKind(t, result.Err, kind)
				if result.Diverged || result.Summary.Completed != 0 || hits.Load() != 2 {
					t.Fatalf("result=%+v hits=%d", result, hits.Load())
				}
			case <-time.After(3 * time.Second):
				t.Fatal("runner did not finish")
			}
		})
	}
}
