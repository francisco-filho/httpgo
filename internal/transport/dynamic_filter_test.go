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
	"capimgrosso.com/httpgo/internal/template"
)

// GO-412: bind/evaluate only the complete final response. An unavailable operand
// proves that incomplete responses and cancellation fail before query preparation.
func TestDynamicFilterFinalResponseAndCancellation(t *testing.T) {
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
						_, _ = io.WriteString(w, `[{"key":1,"id":"wrong"}]`)
						return
					}
					if r.URL.Path == "/next" {
						if capture {
							body, _ := io.ReadAll(r.Body)
							if string(body) != `"final"` {
								t.Errorf("body=%s", body)
							}
						}
						return
					}
					if mode != "final" {
						w.Header().Set("Content-Length", "200")
					}
					_, _ = io.WriteString(w, `[{"key":1,"id":"final"}]`)
					if mode == "timeout" || mode == "cancel" {
						w.(http.Flusher).Flush()
						close(received)
						<-r.Context().Done()
						close(closed)
					}
				}))
				defer server.Close()
				lines := "[Asserts]\n" + `jsonpath "$[?(@.key=={{key}})].id" == "final"`
				next := "GET " + server.URL + "/next"
				if capture {
					lines = "[Captures]\n" + `selected: jsonpath "$[?(@.key=={{key}})].id"`
					next = "POST " + server.URL + "/next\n{{selected}}"
				}
				s, err := parser.ParseWithOptions("dynamic.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP 200\n"+lines+"\n"+next), parser.ParseOptions{RenderAsserts: true})
				if err != nil {
					t.Fatal(err)
				}
				assignments := []string{}
				if mode == "final" {
					assignments = []string{"key=1"}
				}
				c, _ := template.New(assignments, nil)
				executor := NewNetHTTPExecutor()
				if mode == "timeout" {
					executor.timeout = 100 * time.Millisecond
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				done := make(chan runner.Result, 1)
				go func() { done <- runner.NewWithRenderer(executor, c, nil).Run(ctx, s) }()
				if mode == "cancel" {
					select {
					case <-received:
						cancel()
					case <-ctx.Done():
						t.Fatal("no final response headers")
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
				if got.Diverged || hits.Load() != 2 || got.Summary.Completed != 0 || strings.Contains(got.Err.Error(), "variável ausente") {
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

// GO-412–413: read/close failures are transport errors, even after valid JSON;
// resources are closed before presentation and a failed writer stops advancement.
func TestDynamicFilterReadCloseBeforeReport(t *testing.T) {
	for _, mode := range []string{"success", "read", "close"} {
		t.Run(mode, func(t *testing.T) {
			reader := strings.NewReader(`[{"key":1,"id":"final"}]`)
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
			s, err := parser.ParseWithOptions("dynamic.hurl", strings.NewReader("GET http://example.test\nHTTP 200\n[Asserts]\n"+`jsonpath "$[?(@.key=={{key}})].id" == "final"`+"\nGET http://example.test/never"), parser.ParseOptions{RenderAsserts: true})
			if err != nil {
				t.Fatal(err)
			}
			c, _ := template.New([]string{"key=1"}, nil)
			got := runner.NewWithRenderer(regexCaptureExecutor{executor: executor, t: t, retain: true}, c, func(runner.EntryResult) error {
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
