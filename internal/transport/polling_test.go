package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestPollingRedirectsStartAtOriginalRequest(t *testing.T) {
	for _, redirect := range []int{302, 307} {
		t.Run(http.StatusText(redirect), func(t *testing.T) {
			var mu sync.Mutex
			var requests []string
			finals := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				mu.Lock()
				defer mu.Unlock()
				requests = append(requests, r.Method+" "+r.URL.Path+" "+string(data))
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(redirect)
					return
				}
				finals++
				if finals == 1 {
					w.WriteHeader(503)
				} else {
					w.WriteHeader(200)
				}
			}))
			defer server.Close()
			scenario, err := parser.Parse("polling.hurl", strings.NewReader("POST "+server.URL+"/start\n[Options]\nretry: 1\nretry-interval: 0\n{}\nHTTP 200\n"))
			if err != nil {
				t.Fatal(err)
			}
			reports := 0
			got := runner.New(NewNetHTTPExecutor(), func(runner.EntryResult) error { reports++; return nil }).Run(context.Background(), scenario)
			final := "GET /final "
			if redirect == 307 {
				final = "POST /final {}"
			}
			mu.Lock()
			defer mu.Unlock()
			if got.Err != nil || got.Diverged || reports != 1 || got.Attempts != (runner.AttemptSummary{Started: 2, Completed: 2, StatusChecks: 2}) || !reflect.DeepEqual(requests, []string{"POST /start {}", final, "POST /start {}", final}) {
				t.Fatalf("result=%+v reports=%d requests=%v", got, reports, requests)
			}
		})
	}
}

func TestPollingEachAttemptHasFreshDeadline(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		hits++
		if hits == 1 {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(200)
		}
	}))
	defer server.Close()
	executor := NewNetHTTPExecutor()
	var contexts []context.Context
	var deadlines []time.Time
	executor.transport.Proxy = func(req *http.Request) (*url.URL, error) {
		ctx := req.Context()
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("missing HTTP deadline")
		}
		contexts = append(contexts, ctx)
		deadlines = append(deadlines, deadline)
		return nil, nil
	}
	scenario, err := parser.Parse("polling.hurl", strings.NewReader("GET "+server.URL+"\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := runner.New(executor, nil).RunWithOptions(context.Background(), scenario, runner.Options{Timeout: time.Second})
	if got.Err != nil || got.Diverged || len(deadlines) != 2 || !deadlines[1].After(deadlines[0]) {
		t.Fatalf("result=%+v deadlines=%v", got, deadlines)
	}
	for _, ctx := range contexts {
		if ctx.Err() == nil {
			t.Fatal("attempt context retained")
		}
	}
}

func TestPollingIncompleteOrTimedOutResponseDoesNotRetry(t *testing.T) {
	for _, mode := range []string{"incomplete", "timeout", "caller"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			hits := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				hits++
				mu.Unlock()
				if mode == "incomplete" {
					w.Header().Set("Content-Length", "100")
					_, _ = io.WriteString(w, "short")
					return
				}
				w.WriteHeader(503)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer server.Close()
			scenario, err := parser.Parse("polling.hurl", strings.NewReader("GET "+server.URL+"\n[Options]\nretry: 3\nretry-interval: 0\nHTTP 200\n"))
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			timeout := 100 * time.Millisecond
			if mode == "caller" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
				timeout = time.Hour
			}
			got := runner.New(NewNetHTTPExecutor(), nil).RunWithOptions(ctx, scenario, runner.Options{Timeout: timeout})
			mu.Lock()
			defer mu.Unlock()
			if got.Err == nil || got.Diverged || hits != 1 || got.Attempts != (runner.AttemptSummary{Started: 1}) || got.Summary.Completed != 0 {
				t.Fatalf("result=%+v hits=%d", got, hits)
			}
		})
	}
}
