package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestConfiguredTimeoutOwnsWholeOperation(t *testing.T) {
	for _, phase := range []string{"headers", "body", "redirect body"} {
		t.Run(phase, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" && phase != "redirect body" {
					w.Header().Set("Location", "/wait")
					w.WriteHeader(302)
					return
				}
				if phase == "body" {
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				}
				if phase == "redirect body" {
					w.Header().Set("Location", "/never")
					w.WriteHeader(302)
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer server.Close()
			executor := NewNetHTTPExecutor()
			executor.timeout = time.Nanosecond // Explicit request metadata, not shared state, must win.
			var deadlines []time.Time
			executor.transport.Proxy = func(req *http.Request) (*url.URL, error) {
				deadline, _ := req.Context().Deadline()
				deadlines = append(deadlines, deadline)
				return nil, nil
			}
			scenario := parser.Scenario{Entries: []parser.Entry{{URL: server.URL + "/start"}, {URL: server.URL + "/next"}}}
			result := runner.New(executor, nil).RunWithOptions(context.Background(), scenario, runner.Options{Timeout: 75 * time.Millisecond})
			if !errors.Is(result.Err, context.DeadlineExceeded) || result.Summary.Started != 1 || result.Summary.Completed != 0 {
				t.Fatalf("result=%+v", result)
			}
			want := int32(2)
			if phase == "redirect body" {
				want = 1
			}
			if hits.Load() != want || len(deadlines) != int(want) {
				t.Fatalf("hits=%d deadlines=%v", hits.Load(), deadlines)
			}
			for _, deadline := range deadlines {
				if deadline.IsZero() || !deadline.Equal(deadlines[0]) {
					t.Fatal("redirect renewed deadline")
				}
			}
			if executor.timeout != time.Nanosecond {
				t.Fatal("executor mutated")
			}
		})
	}
}

func TestTimeoutMetadataIsolationAndCaller(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/blocked" {
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	executor := NewNetHTTPExecutor()
	if executor.transport.TLSHandshakeTimeout != 0 || executor.transport.ResponseHeaderTimeout != 0 {
		t.Fatal("fixed internal deadline")
	}
	var deadlines []time.Time
	executor.transport.Proxy = func(req *http.Request) (*url.URL, error) {
		deadline, _ := req.Context().Deadline()
		deadlines = append(deadlines, deadline)
		return nil, nil
	}
	for _, limit := range []time.Duration{time.Second, 20 * time.Second, 0} {
		start := time.Now()
		response, err := executor.Execute(context.Background(), runner.Request{URL: server.URL, Timeout: limit})
		if err != nil || response.Status != 200 {
			t.Fatalf("response=%+v err=%v", response, err)
		}
		expected := limit
		if expected == 0 {
			expected = 10 * time.Second
		}
		delta := deadlines[len(deadlines)-1].Sub(start)
		if delta < expected || delta > expected+time.Second {
			t.Fatalf("delta=%s want=%s", delta, expected)
		}
	}
	if !deadlines[1].After(deadlines[0]) {
		t.Fatal("new request did not receive new budget")
	}
	caller, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := executor.Execute(caller, runner.Request{URL: server.URL + "/blocked", Timeout: time.Hour})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("caller deadline lost: %v", err)
	}
	deadline, _ := caller.Deadline()
	if !deadlines[len(deadlines)-1].Equal(deadline) {
		t.Fatal("caller deadline extended")
	}
	ctx, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	_, err = executor.Execute(ctx, runner.Request{URL: server.URL, Timeout: time.Hour})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
