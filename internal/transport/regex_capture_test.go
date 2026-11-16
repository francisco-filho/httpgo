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

func TestRegexCapturesRequireCompleteFinalResponse(t *testing.T) {
	for _, origin := range []string{"body", "header"} {
		for _, mode := range []string{"final", "intermediate only", "incomplete", "timeout", "cancel"} {
			t.Run(origin+"/"+mode, func(t *testing.T) {
				var hits atomic.Int32
				received := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits.Add(1)
					switch r.URL.Path {
					case "/start":
						w.Header().Set("Set-Cookie", "refresh=intermediate; HttpOnly")
						w.Header().Set("Location", "/final")
						w.WriteHeader(307)
						_, _ = io.WriteString(w, "code=intermediate")
					case "/final":
						text := "code=final"
						if mode == "intermediate only" {
							text = "no code"
						} else {
							w.Header().Set("Set-Cookie", "refresh=final; HttpOnly")
						}
						if mode == "incomplete" || mode == "timeout" || mode == "cancel" {
							w.Header().Set("Content-Length", "100")
						}
						_, _ = io.WriteString(w, text)
						if mode == "timeout" || mode == "cancel" {
							w.(http.Flusher).Flush()
							close(received)
							<-r.Context().Done()
						}
					case "/next":
						if r.Header.Get("X-Value") != "final" || r.Header.Get("Cookie") != "" {
							t.Error("wrong final capture or implicit cookie")
						}
					default:
						t.Error("unexpected request")
					}
				}))
				defer server.Close()
				decl := `x: regex "code=(\\w+)"`
				if origin == "header" {
					decl = `x: header "Set-Cookie" regex "refresh=([^;]+)"`
				}
				scenario, err := parser.Parse("regex.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP 200\n[Captures]\n"+decl+"\nGET "+server.URL+"/next\nX-Value: {{x}}\nHTTP 200"))
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
				go func() { done <- runner.New(executor, nil).Run(ctx, scenario) }()
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
					t.Fatal("response not closed")
				}
				wantHits := int32(2)
				switch mode {
				case "final":
					wantHits = 3
					if got.Err != nil || got.Summary.Completed != 2 {
						t.Fatalf("%+v", got)
					}
				case "intermediate only":
					var captureErr *runner.CaptureError
					if !errors.As(got.Err, &captureErr) || got.Summary.Completed != 1 {
						t.Fatalf("%+v", got)
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
					if got.Summary.Completed != 0 {
						t.Fatalf("%+v", got)
					}
				}
				if hits.Load() != wantHits {
					t.Fatalf("hits=%d want=%d", hits.Load(), wantHits)
				}
			})
		}
	}
}

func TestRegexCaptureConsumesAndClosesBeforeEvaluation(t *testing.T) {
	for _, origin := range []string{"body", "header"} {
		for _, mode := range []string{"success", "read", "close"} {
			t.Run(origin+"/"+mode, func(t *testing.T) {
				reader := strings.NewReader("code=private-group")
				body := &captureBody{Reader: reader}
				if mode == "read" {
					body.Reader = io.MultiReader(reader, errorReader{})
				}
				if mode == "close" {
					body.closeErr = errors.New("private close failure")
				}
				executor := NewNetHTTPExecutor()
				calls := 0
				executor.transport.RegisterProtocol("capture", captureRoundTripper(func(*http.Request) (*http.Response, error) {
					calls++
					return &http.Response{StatusCode: 200, Header: http.Header{"Set-Cookie": {"refresh=private-group; HttpOnly"}}, Body: body}, nil
				}))
				decl := `x: regex "code=(.+)"`
				if origin == "header" {
					decl = `x: header "Set-Cookie" regex "refresh=([^;]+)"`
				}
				scenario, err := parser.Parse("regex.hurl", strings.NewReader("GET http://example.test/\nHTTP 200\n[Captures]\n"+decl+"\nGET http://example.test/next"))
				if err != nil {
					t.Fatal(err)
				}
				adapter := regexCaptureExecutor{executor: executor, t: t, retain: origin == "body"}
				got := runner.New(adapter, func(e runner.EntryResult) error {
					if !body.closed || reader.Len() != 0 {
						t.Error("evaluated before consuming and closing")
					}
					if e.Entry.Captures != nil {
						t.Error("operational metadata retained")
					}
					return errors.New("stop")
				}).Run(context.Background(), scenario)
				if calls != 1 || !body.closed || got.ReportErr == nil {
					t.Fatalf("%+v calls=%d", got, calls)
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
}

type regexCaptureExecutor struct {
	executor *NetHTTPExecutor
	t        *testing.T
	retain   bool
}

func (e regexCaptureExecutor) Execute(ctx context.Context, req runner.Request) (runner.Response, error) {
	e.t.Helper()
	if req.RetainBody != e.retain {
		e.t.Fatalf("retain=%v want=%v", req.RetainBody, e.retain)
	}
	req.URL = "capture://local"
	response, err := e.executor.Execute(ctx, req)
	if !e.retain && response.Body != nil {
		e.t.Fatal("header-only retains body")
	}
	return response, err
}
