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

func TestHeaderCapturesUseOnlyCompleteFinalResponse(t *testing.T) {
	for _, mode := range []string{"final", "missing", "timeout", "cancel", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			received := make(chan struct{})
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/start":
					w.Header().Set("ETag", `"intermediate"`)
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
				case "/final":
					if mode != "missing" {
						w.Header().Set("ETag", `"final"`)
					}
					w.Header().Set("Set-Cookie", "session=private-response")
					if mode == "incomplete" || mode == "timeout" || mode == "cancel" {
						w.Header().Set("Content-Length", "100")
					}
					w.WriteHeader(200)
					_, _ = io.WriteString(w, "not JSON")
					if mode == "timeout" || mode == "cancel" {
						w.(http.Flusher).Flush()
						close(received)
						<-r.Context().Done()
					}
				case "/next":
					if r.Header.Get("If-None-Match") != `"final"` || r.Header.Get("Cookie") != "" {
						t.Error("intermediate capture or implicit cookie")
					}
				default:
					t.Error("unexpected request")
				}
			}))
			defer server.Close()
			scenario, err := parser.Parse("header.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP 200\n[Captures]\nx: header \"ETag\"\nGET "+server.URL+"/next\nIf-None-Match: {{x}}\nHTTP 200"))
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
			go func() {
				done <- runner.New(executor, func(e runner.EntryResult) error {
					if e.Entry.Headers != nil || e.Entry.Captures != nil || e.Entry.Body != nil {
						t.Error("callback retains operational data")
					}
					return nil
				}).Run(ctx, scenario)
			}()
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
				t.Fatal("response consumption did not finish")
			}
			switch mode {
			case "final":
				if got.Err != nil || got.Summary.Completed != 2 || hits.Load() != 3 {
					t.Fatalf("%+v hits=%d", got, hits.Load())
				}
			case "missing":
				var captureErr *runner.CaptureError
				if !errors.As(got.Err, &captureErr) || got.Summary.Completed != 1 || hits.Load() != 2 {
					t.Fatalf("%+v hits=%d", got, hits.Load())
				}
			default:
				kind := Body
				if mode == "timeout" {
					kind = Timeout
				}
				if mode == "cancel" {
					kind = Canceled
				}
				requireKind(t, got.Err, kind)
				if got.Summary.Completed != 0 || hits.Load() != 2 || got.Entries[0].Outcome != runner.Failed {
					t.Fatalf("%+v hits=%d", got, hits.Load())
				}
			}
		})
	}
}

func TestHeaderOnlyCaptureConsumesClosesAndDiscardsBody(t *testing.T) {
	for _, mode := range []string{"success", "read", "close"} {
		t.Run(mode, func(t *testing.T) {
			reader := strings.NewReader(strings.Repeat("not JSON", 1000))
			body := &captureBody{Reader: reader}
			if mode == "read" {
				body.Reader = io.MultiReader(reader, errorReader{})
			}
			if mode == "close" {
				body.closeErr = errors.New("private close failure")
			}
			executor := NewNetHTTPExecutor()
			calls, reports := 0, 0
			executor.transport.RegisterProtocol("capture", captureRoundTripper(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Header: http.Header{"Etag": {`"exact"`}}, Body: body}, nil
			}))
			scenario, err := parser.Parse("header.hurl", strings.NewReader("GET http://example.test/\nHTTP 200\n[Captures]\nx: header \"ETag\"\nGET http://example.test/next"))
			if err != nil {
				t.Fatal(err)
			}
			adapter := headerCaptureExecutor{executor: executor, t: t}
			got := runner.New(adapter, func(e runner.EntryResult) error {
				reports++
				if !body.closed || reader.Len() != 0 {
					t.Error("evaluated before consuming and closing body")
				}
				return errors.New("stop after first report")
			}).Run(context.Background(), scenario)
			if calls != 1 || reports != 1 || !body.closed || got.ReportErr == nil {
				t.Fatalf("%+v calls=%d reports=%d", got, calls, reports)
			}
			if mode == "success" {
				if got.Err != nil || got.Summary.Completed != 1 {
					t.Fatalf("%+v", got)
				}
			} else {
				requireKind(t, got.Err, Body)
				if got.Summary.Completed != 0 {
					t.Fatalf("%+v", got)
				}
			}
		})
	}
}

type headerCaptureExecutor struct {
	executor *NetHTTPExecutor
	t        *testing.T
}

func (e headerCaptureExecutor) Execute(ctx context.Context, req runner.Request) (runner.Response, error) {
	e.t.Helper()
	if req.RetainBody {
		e.t.Fatal("header-only capture requests retention")
	}
	req.URL = "capture://local"
	response, err := e.executor.Execute(ctx, req)
	if response.Body != nil {
		e.t.Fatal("header-only response retains body")
	}
	return response, err
}
