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

func TestCaptureRealHTTPAndFinalRetention(t *testing.T) {
	for _, retain := range []bool{false, true} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			switch r.URL.Path {
			case "/login":
				w.Header().Set("Location", "/final")
				w.WriteHeader(307)
				_, _ = io.WriteString(w, `{"token":"wrong"}`)
			case "/final":
				_, _ = io.WriteString(w, `{"token":"actual"}`)
			case "/products":
				if r.Header.Get("Authorization") != "Bearer actual" {
					t.Error("token not sent")
				}
				w.WriteHeader(204)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		}))
		executor := NewNetHTTPExecutor()
		response, err := executor.Execute(context.Background(), runner.Request{URL: server.URL + "/login", RetainBody: retain})
		if err != nil || response.Status != 200 || retain && string(response.Body) != `{"token":"actual"}` || !retain && response.Body != nil {
			t.Fatalf("%+v %v", response, err)
		}
		s, err := parser.Parse("login.hurl", strings.NewReader("POST "+server.URL+"/login\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\nGET "+server.URL+"/products\nAuthorization: Bearer {{token}}\nHTTP 204"))
		if err != nil {
			t.Fatal(err)
		}
		got := runner.New(executor, nil).Run(context.Background(), s)
		server.Close()
		if got.Err != nil || got.Summary.Validated != 2 || hits.Load() != 5 {
			t.Fatalf("%+v hits=%d", got, hits.Load())
		}
	}
}

type captureRoundTripper func(*http.Request) (*http.Response, error)

func (f captureRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type captureBody struct {
	io.Reader
	closed   bool
	closeErr error
}

func (b *captureBody) Close() error { b.closed = true; return b.closeErr }

func TestRetainedBodyReadAndCloseErrors(t *testing.T) {
	for _, mode := range []string{"read", "close", "success"} {
		b := &captureBody{Reader: strings.NewReader(`{"id":9007199254740993,"nested":[null,true,1.2300e+99]}`)}
		if mode == "close" {
			b.closeErr = errors.New("secret close cause")
		}
		if mode == "read" {
			b.Reader = io.MultiReader(strings.NewReader(`{"id":9007199254740993,"nested":[null,true,1.2300e+99]}`), errorReader{})
		}
		executor := NewNetHTTPExecutor()
		executor.transport.RegisterProtocol("capture", captureRoundTripper(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: b, Header: make(http.Header)}, nil
		}))
		got, err := executor.Execute(context.Background(), runner.Request{URL: "capture://local", RetainBody: true})
		if !b.closed {
			t.Fatal("body left open")
		}
		if mode == "success" {
			if err != nil || string(got.Body) != `{"id":9007199254740993,"nested":[null,true,1.2300e+99]}` {
				t.Fatalf("%+v %v", got, err)
			}
		} else {
			requireKind(t, err, Body)
			if got.Body != nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("%+v %v", got, err)
			}
		}
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("secret read cause") }

func TestCaptureReadDeadlineAndCancellation(t *testing.T) {
	for _, mode := range []string{"timeout", "cancel", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			received := make(chan struct{})
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/login" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
					return
				}
				if mode == "incomplete" {
					w.Header().Set("Content-Length", "100")
					_, _ = io.WriteString(w, `{"id":9007199254740993}`)
					return
				}
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(200)
				_, _ = io.WriteString(w, `{"id":9007199254740993}`)
				w.(http.Flusher).Flush()
				close(received)
				<-r.Context().Done()
			}))
			defer server.Close()
			s, err := parser.Parse("login.hurl", strings.NewReader("GET "+server.URL+"/login\n[Captures]\ntoken: jsonpath \"$\"\nGET "+server.URL+"/next"))
			if err != nil {
				t.Fatal(err)
			}
			executor := NewNetHTTPExecutor()
			executor.timeout = 100 * time.Millisecond
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan runner.Result, 1)
			go func() { done <- runner.New(executor, nil).Run(ctx, s) }()
			if mode == "cancel" {
				select {
				case <-received:
					cancel()
				case <-time.After(3 * time.Second):
					t.Fatal("no response")
				}
			}
			var got runner.Result
			select {
			case got = <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("Run did not finish")
			}
			kind := Timeout
			if mode == "cancel" {
				kind = Canceled
			}
			if mode == "incomplete" {
				kind = Body
			}
			requireKind(t, got.Err, kind)
			if hits.Load() != 2 || got.Summary.Completed != 0 || got.Entries[0].Outcome != runner.Failed {
				t.Fatalf("%+v hits=%d", got, hits.Load())
			}
		})
	}
}
