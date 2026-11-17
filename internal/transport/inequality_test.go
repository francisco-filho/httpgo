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

// GO-428: even a valid, different partial value cannot approve before read/close.
func TestJSONInequalityRequiresReadAndClose(t *testing.T) {
	for _, mode := range []string{"success", "read", "close"} {
		t.Run(mode, func(t *testing.T) {
			body := &captureBody{Reader: strings.NewReader(`{"token":"new-private"}`)}
			if mode == "read" {
				body.Reader = io.MultiReader(body.Reader, errorReader{})
			}
			if mode == "close" {
				body.closeErr = errors.New("private close cause")
			}
			executor := NewNetHTTPExecutor()
			calls, reports := 0, 0
			executor.transport.RegisterProtocol("http", captureRoundTripper(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
			}))
			s, err := parser.Parse("inequality.hurl", strings.NewReader("GET http://example.test\nHTTP 200\n[Asserts]\njsonpath \"$.token\" != \"old-private\"\n[Captures]\nnew: jsonpath \"$.token\"\nGET http://example.test/{{new}}"))
			if err != nil {
				t.Fatal(err)
			}
			got := runner.New(executor, func(runner.EntryResult) error {
				reports++
				if !body.closed {
					t.Fatal("reported before close")
				}
				return nil
			}).Run(context.Background(), s)
			if !body.closed || reports != calls {
				t.Fatalf("closed=%v reports=%d calls=%d", body.closed, reports, calls)
			}
			if mode == "success" {
				if got.Err != nil || got.Diverged || calls != 2 {
					t.Fatalf("result=%+v calls=%d", got, calls)
				}
			} else {
				requireKind(t, got.Err, Body)
				if got.Diverged || calls != 1 || got.Summary.Completed != 0 || got.Entries[0].JSONFailure != nil {
					t.Fatalf("result=%+v calls=%d", got, calls)
				}
			}
		})
	}
}

// GO-428: redirect content is irrelevant; cancellation/timeout interrupts consumption.
func TestJSONInequalityFinalAndCancellation(t *testing.T) {
	for _, mode := range []string{"final", "incomplete", "timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ready := make(chan struct{})
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(302)
					_, _ = io.WriteString(w, `"old"`)
					return
				}
				if r.URL.Path == "/next" {
					return
				}
				if mode != "final" {
					w.Header().Set("Content-Length", "100")
				}
				_, _ = io.WriteString(w, `"new"`)
				if mode == "timeout" || mode == "cancel" {
					w.(http.Flusher).Flush()
					close(ready)
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			s, err := parser.Parse("inequality.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP 200\n[Asserts]\njsonpath \"$\" != \"old\"\n[Captures]\nnew: jsonpath \"$\"\nGET "+server.URL+"/next"))
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
					t.Fatal("response did not start")
				}
			}
			select {
			case got := <-done:
				if mode == "final" {
					if got.Err != nil || got.Diverged || hits.Load() != 3 {
						t.Fatalf("result=%+v hits=%d", got, hits.Load())
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
				requireKind(t, got.Err, kind)
				if got.Diverged || got.Summary.Completed != 0 || hits.Load() != 2 {
					t.Fatalf("result=%+v hits=%d", got, hits.Load())
				}
			case <-time.After(3 * time.Second):
				t.Fatal("runner did not terminate")
			}
		})
	}
}
