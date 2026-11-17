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

func countCaptureScenario(t *testing.T, base string) parser.Scenario {
	t.Helper()
	s, err := parser.Parse("count.hurl", strings.NewReader("GET "+base+"/start\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$\" count\nPOST "+base+"/next\n{\"count\":{{baseline}}}\nHTTP 200"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// GO-442: count only complete final responses, and close stalled responses.
func TestCountCaptureFinalResponse(t *testing.T) {
	for _, mode := range []string{"final", "incomplete", "timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			received, closed := make(chan struct{}), make(chan struct{})
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/start":
					w.Header().Set("Location", "/final")
					w.WriteHeader(302)
					_, _ = io.WriteString(w, `[1]`)
				case "/next":
					body, err := io.ReadAll(r.Body)
					if err != nil || string(body) != `{"count":3}` {
						t.Errorf("body=%s err=%v", body, err)
					}
				case "/final":
					if mode != "final" {
						w.Header().Set("Content-Length", "100")
					}
					_, _ = io.WriteString(w, `[null,null,null]`)
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
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			done := make(chan runner.Result, 1)
			scenario := countCaptureScenario(t, server.URL)
			go func() { done <- runner.New(executor, nil).Run(ctx, scenario) }()
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
				if got.Diverged || got.Summary.Completed != 0 || hits.Load() != 2 || got.Entries[0].Outcome != runner.Failed {
					t.Fatalf("partial evaluated: %+v hits=%d", got, hits.Load())
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

func TestCountCaptureReadAndCloseFailure(t *testing.T) {
	for _, mode := range []string{"read", "close"} {
		t.Run(mode, func(t *testing.T) {
			body := &captureBody{Reader: strings.NewReader(`[null,null,null]`)}
			if mode == "read" {
				body.Reader = io.MultiReader(body.Reader, errorReader{})
			} else {
				body.closeErr = errors.New("private close cause")
			}
			executor := NewNetHTTPExecutor()
			calls := 0
			executor.transport.RegisterProtocol("capture", captureRoundTripper(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: body}, nil
			}))
			got := runner.New(regexCaptureExecutor{executor: executor, t: t, retain: true}, nil).Run(context.Background(), countCaptureScenario(t, "http://example.test"))
			requireKind(t, got.Err, Body)
			if calls != 1 || !body.closed || got.Diverged || got.Summary.Completed != 0 || got.Entries[0].Outcome != runner.Failed {
				t.Fatalf("calls=%d closed=%v result=%+v", calls, body.closed, got)
			}
		})
	}
}
