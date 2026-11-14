package transport

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestNewMethodsKeepNetworkPolicies(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		for _, mode := range []string{"timeout", "cancel", "TLS", "incomplete"} {
			t.Run(method+"/"+mode, func(t *testing.T) {
				var hits, next atomic.Int32
				started := make(chan struct{}, 1)
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/next" {
						next.Add(1)
						return
					}
					hits.Add(1)
					_, _ = io.Copy(io.Discard, r.Body)
					if r.URL.Path == "/start" && mode == "timeout" {
						w.Header().Set("Location", "/wait")
						w.WriteHeader(307)
						return
					}
					if mode == "incomplete" {
						w.Header().Set("Content-Length", "100")
						_, _ = io.WriteString(w, "short")
						return
					}
					started <- struct{}{}
					if mode == "timeout" {
						w.WriteHeader(200)
						w.(http.Flusher).Flush()
					}
					<-r.Context().Done()
				}))
				server.Config.ErrorLog = log.New(io.Discard, "", 0)
				if mode == "TLS" {
					server.StartTLS()
				} else {
					server.Start()
				}
				defer server.Close()
				executor := NewNetHTTPExecutor()
				executor.timeout = 150 * time.Millisecond
				var deadlines []time.Time
				executor.transport.Proxy = func(req *http.Request) (*url.URL, error) {
					deadline, _ := req.Context().Deadline()
					deadlines = append(deadlines, deadline)
					return nil, nil
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan runner.Result, 1)
				go func() {
					done <- runner.New(executor, nil).Run(ctx, parser.Scenario{Entries: []parser.Entry{{Method: method, URL: server.URL + "/start", Body: []byte(`{"a":1}`)}, {Method: "DELETE", URL: server.URL + "/next"}}})
				}()
				if mode == "cancel" {
					select {
					case <-started:
						cancel()
					case <-time.After(3 * time.Second):
						t.Fatal("request not started")
					}
				}
				result := <-done
				kind, wantHits := Body, int32(1)
				switch mode {
				case "timeout":
					kind, wantHits = Timeout, 2
				case "cancel":
					kind = Canceled
				case "TLS":
					kind, wantHits = TLS, 0
				}
				requireKind(t, result.Err, kind)
				if hits.Load() != wantHits || next.Load() != 0 || result.Summary.Started != 1 || result.Summary.Completed != 0 {
					t.Fatalf("result=%+v,hits=%d,next=%d", result, hits.Load(), next.Load())
				}
				for _, deadline := range deadlines {
					if deadline.IsZero() || !deadline.Equal(deadlines[0]) {
						t.Fatalf("deadline changed: %v", deadlines)
					}
				}
			})
		}
	}
}
