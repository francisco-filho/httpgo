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
	"capimgrosso.com/httpgo/internal/template"
)

func dynamicResponseScenario(t *testing.T, base string) parser.Scenario {
	t.Helper()
	source := "GET " + base + "/start\nHTTP 200\n[Asserts]\nheader \"X-Test\" matches \"^{{value}}$\"\njsonpath \"$\" == \"{{value}}\"\n[Captures]\nx: header \"ETag\"\nGET " + base + "/next\nIf-None-Match: {{x}}"
	s, err := parser.ParseWithOptions("dynamic.hurl", strings.NewReader(source), parser.ParseOptions{RenderAsserts: true})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// GO-394: response final only; read/timeout/cancel cannot trigger expected resolution.
func TestRenderAssertsCompleteFinalResponse(t *testing.T) {
	for _, mode := range []string{"final", "incomplete", "timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			received, closed := make(chan struct{}), make(chan struct{})
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/start":
					w.Header().Set("X-Test", "intermediate")
					w.Header().Set("Location", "/final")
					w.WriteHeader(302)
				case "/next":
					if r.Header.Get("If-None-Match") != "captured" {
						t.Error("capture unavailable")
					}
				case "/final":
					w.Header().Set("X-Test", "final")
					w.Header().Set("ETag", "captured")
					if mode != "final" {
						w.Header().Set("Content-Length", "100")
					}
					_, _ = io.WriteString(w, `"final"`)
					if mode == "timeout" || mode == "cancel" {
						w.(http.Flusher).Flush()
						close(received)
						<-r.Context().Done()
						close(closed)
					}
				}
			}))
			defer server.Close()
			executor := NewNetHTTPExecutor()
			if mode == "timeout" {
				executor.timeout = 150 * time.Millisecond
			}
			c, _ := template.New([]string{"value=final"}, nil)
			// Missing variable would win if preparation ran on partial responses.
			if mode != "final" {
				c, _ = template.New(nil, nil)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			done := make(chan runner.Result, 1)
			scenario := dynamicResponseScenario(t, server.URL)
			go func() {
				done <- runner.NewWithRenderer(executor, c, nil).Run(ctx, scenario)
			}()
			if mode == "cancel" {
				select {
				case <-received:
					cancel()
				case <-ctx.Done():
					t.Fatal("no response")
				}
			}
			var got runner.Result
			select {
			case got = <-done:
			case <-time.After(4 * time.Second):
				t.Fatal("execution stuck")
			}
			if mode == "final" {
				if got.Err != nil || got.Diverged || got.Summary.Completed != 2 || hits.Load() != 3 {
					t.Fatalf("result=%+v hits=%d", got, hits.Load())
				}
			} else {
				kind := Body
				if mode == "timeout" {
					kind = Timeout
				}
				if mode == "cancel" {
					kind = Canceled
				}
				requireKind(t, got.Err, kind)
				if got.Diverged || hits.Load() != 2 || got.Summary.Completed != 0 || got.Entries[0].HeaderError != nil || got.Entries[0].JSONError != nil {
					t.Fatalf("partial evaluation: %+v", got)
				}
				if mode == "timeout" || mode == "cancel" {
					select {
					case <-closed:
					case <-time.After(time.Second):
						t.Fatal("connection not closed")
					}
				}
			}
		})
	}
}

func TestRenderAssertsReadAndCloseFailure(t *testing.T) {
	for _, mode := range []string{"read", "close"} {
		t.Run(mode, func(t *testing.T) {
			body := &captureBody{Reader: strings.NewReader(`"final"`)}
			if mode == "read" {
				body.Reader = io.MultiReader(body.Reader, errorReader{})
			} else {
				body.closeErr = errors.New("private close cause")
			}
			executor := NewNetHTTPExecutor()
			calls := 0
			executor.transport.RegisterProtocol("capture", captureRoundTripper(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Header: http.Header{"X-Test": {"final"}}, Body: body}, nil
			}))
			got := runner.New(regexCaptureExecutor{executor: executor, t: t, retain: true}, nil).Run(context.Background(), dynamicResponseScenario(t, "http://example.test"))
			requireKind(t, got.Err, Body)
			if calls != 1 || !body.closed || got.Diverged || got.Summary.Completed != 0 || got.Entries[0].HeaderError != nil {
				t.Fatalf("calls=%d closed=%v result=%+v", calls, body.closed, got)
			}
		})
	}
}
