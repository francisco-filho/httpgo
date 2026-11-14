package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestRedirectLimitAndFinalStatus(t *testing.T) {
	for _, hops := range []int{0, 1, 10, 11} {
		t.Run(strconv.Itoa(hops), func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
				if n < hops {
					w.Header().Set("Location", fmt.Sprintf("/%d", n+1))
					w.WriteHeader(302)
					return
				}
				w.WriteHeader(404)
			}))
			defer server.Close()
			got, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{URL: server.URL + "/0"})
			if hops == 11 {
				requireKind(t, err, Redirect)
				if hits.Load() != 11 {
					t.Fatalf("hits = %d, want 11", hits.Load())
				}
			} else if err != nil || got.Status != 404 || hits.Load() != int32(hops+1) {
				t.Fatalf("response = %+v, error = %v, hits = %d", got, err, hits.Load())
			}
		})
	}
}

func TestOrigin(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"http://EXAMPLE.test", "HTTP://example.TEST:80/next", true},
		{"https://example.test", "https://example.test:443/next", true},
		{"http://example.test", "https://example.test:80", false},
		{"http://example.test", "http://other.test", false},
		{"http://example.test", "http://example.test:81", false},
		{"http://localhost", "http://127.0.0.1", false},
	} {
		a, _ := url.Parse(tc.a)
		b, _ := url.Parse(tc.b)
		if got := sameOrigin(a, b); got != tc.want {
			t.Fatalf("sameOrigin(%q,%q) = %v", tc.a, tc.b, got)
		}
	}
}

func TestForbiddenRedirectNeverContactsTarget(t *testing.T) {
	var contacts atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { contacts.Add(1); w.WriteHeader(200) }))
	defer target.Close()
	for _, kind := range []string{"port", "host", "scheme", "invalid"} {
		t.Run(kind, func(t *testing.T) {
			var subsequent atomic.Int32
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/next" {
					subsequent.Add(1)
					return
				}
				destination := target.URL + "/private?token=destination-secret"
				switch kind {
				case "host":
					destination = strings.Replace(destination, "127.0.0.1", "localhost", 1)
				case "scheme":
					destination = strings.Replace(destination, "http:", "https:", 1)
				case "invalid":
					destination = "http://bad%zz/?token=destination-secret"
				}
				w.Header().Set("Location", destination)
				w.WriteHeader(302)
			}))
			defer source.Close()
			result := runner.New(NewNetHTTPExecutor()).Run(context.Background(), parser.Scenario{Entries: []parser.Entry{{URL: source.URL}, {URL: source.URL + "/next"}}})
			requireKind(t, result.Err, Redirect)
			if strings.Contains(result.Err.Error(), "destination-secret") || contacts.Load() != 0 || subsequent.Load() != 0 || result.Summary.Started != 1 {
				t.Fatalf("unsafe redirect: %+v, target=%d,next=%d", result, contacts.Load(), subsequent.Load())
			}
		})
	}
}

func TestTLSAndConnectionFailure(t *testing.T) {
	tlsServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	tlsServer.Config.ErrorLog = log.New(io.Discard, "", 0)
	tlsServer.StartTLS()
	defer tlsServer.Close()
	_, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{URL: tlsServer.URL})
	requireKind(t, err, TLS)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = NewNetHTTPExecutor().Execute(context.Background(), runner.Request{URL: "http://" + address})
	requireKind(t, err, Network)
}

func TestTotalDeadlineIncludesRedirectsAndBody(t *testing.T) {
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
				switch phase {
				case "redirect body":
					w.Header().Set("Location", "/never")
					w.WriteHeader(302)
					w.(http.Flusher).Flush()
				case "body":
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer server.Close()
			executor := NewNetHTTPExecutor()
			executor.timeout = 150 * time.Millisecond
			var mu sync.Mutex
			var deadlines []time.Time
			executor.transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				deadline, _ := ctx.Deadline()
				mu.Lock()
				deadlines = append(deadlines, deadline)
				mu.Unlock()
				return (&net.Dialer{}).DialContext(ctx, network, address)
			}
			result := runner.New(executor).Run(context.Background(), parser.Scenario{Entries: []parser.Entry{{URL: server.URL + "/start"}, {URL: server.URL + "/next"}}})
			requireKind(t, result.Err, Timeout)
			if !errors.Is(result.Err, context.DeadlineExceeded) || result.Summary.Completed != 0 || result.Summary.Started != 1 {
				t.Fatalf("result = %+v", result)
			}
			wantHits := int32(2)
			if phase == "redirect body" {
				wantHits = 1
			}
			if hits.Load() != wantHits {
				t.Fatalf("hits=%d,want %d", hits.Load(), wantHits)
			}
			mu.Lock()
			defer mu.Unlock()
			// Transport detaches dialing from the request deadline for pooling.
			// The end-to-end deadline is instead enforced by RoundTrip's context.
			if len(deadlines) != int(wantHits) {
				t.Fatalf("dials=%d,want %d", len(deadlines), wantHits)
			}
		})
	}
}

func TestIncompleteResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(200)
		_, _ = io.WriteString(w, "short")
	}))
	defer server.Close()
	_, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{URL: server.URL})
	requireKind(t, err, Body)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("cause=%v", err)
	}
}

func TestNoImplicitRetryAfterWarmup(t *testing.T) {
	var hits atomic.Int32
	var mu sync.Mutex
	var addresses []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		mu.Lock()
		addresses = append(addresses, r.RemoteAddr)
		mu.Unlock()
		if !r.Close {
			t.Error("request permits connection reuse")
		}
		if n == 2 {
			connection, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = connection.Close()
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	executor := NewNetHTTPExecutor()
	if !executor.transport.DisableKeepAlives || executor.transport.ForceAttemptHTTP2 || executor.transport.TLSNextProto == nil {
		t.Fatal("unsafe retry configuration")
	}
	if _, err := executor.Execute(context.Background(), runner.Request{URL: server.URL}); err != nil {
		t.Fatal(err)
	}
	result := runner.New(executor).Run(context.Background(), parser.Scenario{Entries: []parser.Entry{{URL: server.URL}, {URL: server.URL + "/next"}}})
	requireKind(t, result.Err, Network)
	mu.Lock()
	defer mu.Unlock()
	if hits.Load() != 2 || len(addresses) != 2 || addresses[0] == addresses[1] || result.Summary.Started != 1 {
		t.Fatalf("hits=%d,addresses=%v,result=%+v", hits.Load(), addresses, result)
	}
}

func TestCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := NewNetHTTPExecutor().Execute(ctx, runner.Request{URL: server.URL}); done <- err }()
	<-started
	cancel()
	err := <-done
	requireKind(t, err, Canceled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cause=%v", err)
	}
}

func TestDefaultTimeout(t *testing.T) {
	if got := NewNetHTTPExecutor().timeout; got != 10*time.Second {
		t.Fatalf("timeout=%s", got)
	}
}

func requireKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	var got *Error
	if !errors.As(err, &got) || got.Kind != want {
		t.Fatalf("error=%v,want kind %s", err, want)
	}
}
