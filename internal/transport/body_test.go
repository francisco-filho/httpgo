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

func TestBodyAssertionsRequireReadAndClose(t *testing.T) {
	for _, mode := range []string{"success", "read", "close"} {
		t.Run(mode, func(t *testing.T) {
			body := &captureBody{Reader: strings.NewReader(`{"customer":true,"token":"captured"}`)}
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
			s, err := parser.Parse("body.hurl", strings.NewReader("GET http://example.test\nHTTP *\n[Asserts]\nbody not contains \"passwordHash\"\n[Captures]\ntoken: jsonpath \"$.token\"\nGET http://example.test/{{token}}"))
			if err != nil {
				t.Fatal(err)
			}
			result := runner.New(executor, func(runner.EntryResult) error {
				reports++
				if !body.closed {
					t.Fatal("reported before closing body")
				}
				return nil
			}).Run(context.Background(), s)
			if !body.closed || reports != calls {
				t.Fatalf("closed=%v reports=%d calls=%d", body.closed, reports, calls)
			}
			if mode == "success" {
				if result.Err != nil || result.Diverged || calls != 2 {
					t.Fatalf("result=%+v calls=%d", result, calls)
				}
			} else {
				requireKind(t, result.Err, Body)
				if result.Diverged || calls != 1 || result.Summary.Completed != 0 || result.Entries[0].Outcome != runner.Failed {
					t.Fatalf("result=%+v calls=%d", result, calls)
				}
			}
		})
	}
}

func TestBodyFinalResponseAndConsumptionCancellation(t *testing.T) {
	for _, mode := range []string{"success", "timeout", "cancel", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			ready := make(chan struct{})
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("Content-Type", "text/plain; charset=unknown")
					w.Header().Set("Location", "/final")
					w.WriteHeader(302)
					_, _ = io.WriteString(w, "passwordHash\xff")
					return
				}
				w.Header().Set("Content-Type", "text/plain; charset=iso-8859-1")
				if mode != "success" {
					w.Header().Set("Content-Length", "100")
				}
				_, _ = io.WriteString(w, "customer a\xe7\xe3o")
				if mode == "timeout" || mode == "cancel" {
					w.(http.Flusher).Flush()
					close(ready)
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			s, err := parser.Parse("body.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP *\n[Asserts]\nbody contains \"ação\"\nbody not contains \"passwordHash\"\nGET "+server.URL+"/next"))
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
				t.Fatal("runner did not terminate")
			}
		})
	}
}
