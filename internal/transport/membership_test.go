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

func TestMembershipRequiresCompleteClosedResponse(t *testing.T) {
	for _, predicate := range []string{`header "X" not contains "forbidden"`, `jsonpath "$.missing" not exists`, `jsonpath "$.items" not contains "forbidden"`} {
		for _, mode := range []string{"success", "read", "close"} {
			t.Run(predicate+mode, func(t *testing.T) {
				reader := strings.NewReader(`{"items":[],"token":"captured"}`)
				body := &captureBody{Reader: reader}
				if mode == "read" {
					body.Reader = io.MultiReader(reader, errorReader{})
				}
				if mode == "close" {
					body.closeErr = errors.New("private close cause")
				}
				executor := NewNetHTTPExecutor()
				calls, reports := 0, 0
				executor.transport.RegisterProtocol("http", captureRoundTripper(func(*http.Request) (*http.Response, error) {
					calls++
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body}, nil
				}))
				source := "GET http://example.test\nHTTP *\n[Asserts]\n" + predicate + "\nGET http://example.test/never"
				s, err := parser.Parse("membership.hurl", strings.NewReader(source))
				if err != nil {
					t.Fatal(err)
				}
				got := runner.New(membershipRetentionExecutor{executor: executor, t: t, retain: strings.HasPrefix(predicate, "jsonpath")}, func(runner.EntryResult) error {
					reports++
					if !body.closed || reader.Len() != 0 {
						t.Fatal("reported before consumption/close")
					}
					return errors.New("stop at report")
				}).Run(context.Background(), s)
				if calls != 1 || reports != 1 || !body.closed || got.ReportErr == nil {
					t.Fatalf("%+v calls=%d reports=%d", got, calls, reports)
				}
				if mode == "success" {
					if got.Err != nil || got.Diverged || got.Summary.Completed != 1 {
						t.Fatalf("%+v", got)
					}
				} else {
					requireKind(t, got.Err, Body)
					if got.Diverged || got.Summary.Completed != 0 {
						t.Fatalf("%+v", got)
					}
				}
			})
		}
	}
}

type membershipRetentionExecutor struct {
	executor *NetHTTPExecutor
	t        *testing.T
	retain   bool
}

func (e membershipRetentionExecutor) Execute(ctx context.Context, req runner.Request) (runner.Response, error) {
	e.t.Helper()
	if req.RetainBody != e.retain {
		e.t.Fatalf("retention=%v want %v", req.RetainBody, e.retain)
	}
	response, err := e.executor.Execute(ctx, req)
	if !e.retain && response.Body != nil {
		e.t.Fatal("header-only assertion retained body")
	}
	return response, err
}

func TestMembershipFinalResponseAndCancellation(t *testing.T) {
	for _, predicate := range []string{`header "X" not contains "forbidden"`, `jsonpath "$.items" not contains "forbidden"`} {
		for _, mode := range []string{"success", "forbidden final", "timeout", "cancel", "incomplete"} {
			t.Run(predicate+mode, func(t *testing.T) {
				ready := make(chan struct{})
				var hits atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits.Add(1)
					if r.URL.Path == "/start" {
						w.Header().Set("X", "forbidden")
						w.Header().Set("Location", "/final")
						w.WriteHeader(302)
						_, _ = io.WriteString(w, `{"items":["forbidden"]}`)
						return
					}
					body := `{"items":[]}`
					if mode == "forbidden final" {
						w.Header().Set("X", "forbidden")
						body = `{"items":["forbidden"]}`
					}
					if mode == "timeout" || mode == "cancel" || mode == "incomplete" {
						w.Header().Set("Content-Length", "100")
					}
					_, _ = io.WriteString(w, body)
					if mode == "timeout" || mode == "cancel" {
						w.(http.Flusher).Flush()
						close(ready)
						<-r.Context().Done()
					}
				}))
				defer server.Close()
				captureHeader := "Absent"
				if mode == "forbidden final" {
					captureHeader = "X"
				}
				s, err := parser.Parse("membership.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP *\n[Asserts]\n"+predicate+"\n[Captures]\nx: header \""+captureHeader+"\"\nGET "+server.URL+"/never"))
				if err != nil {
					t.Fatal(err)
				}
				executor := NewNetHTTPExecutor()
				if mode == "timeout" {
					executor.timeout = 150 * time.Millisecond
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				done := make(chan runner.Result, 1)
				go func() { done <- runner.New(executor, nil).Run(ctx, s) }()
				if mode == "cancel" {
					select {
					case <-ready:
						cancel()
					case <-ctx.Done():
						t.Fatal("response did not start")
					}
				}
				var got runner.Result
				select {
				case got = <-done:
				case <-time.After(4 * time.Second):
					t.Fatal("runner did not finish")
				}
				if hits.Load() != 2 {
					t.Fatalf("unexpected requests %d", hits.Load())
				}
				switch mode {
				case "success":
					var captureErr *runner.CaptureError
					if !errors.As(got.Err, &captureErr) || got.Diverged || got.Summary.Completed != 1 {
						t.Fatalf("%+v", got)
					}
				case "forbidden final":
					if got.Err != nil || !got.Diverged || got.Summary.Completed != 1 {
						t.Fatalf("%+v", got)
					}
				default:
					kind := Body
					if mode == "cancel" {
						kind = Canceled
					}
					if mode == "timeout" {
						kind = Timeout
					}
					requireKind(t, got.Err, kind)
					if got.Diverged || got.Summary.Completed != 0 || got.Entries[0].Outcome != runner.Failed {
						t.Fatalf("%+v", got)
					}
				}
			})
		}
	}
}
