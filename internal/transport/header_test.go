package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestHeaderBodyReadAndCloseErrors(t *testing.T) {
	for _, mode := range []string{"read", "close", "success"} {
		b := &captureBody{Reader: strings.NewReader("body need not be JSON")}
		if mode == "close" {
			b.closeErr = errors.New("private close cause")
		}
		if mode == "read" {
			b.Reader = io.MultiReader(b.Reader, errorReader{})
		}
		executor := NewNetHTTPExecutor()
		executor.transport.RegisterProtocol("header", captureRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: b, Header: http.Header{"X-Test": []string{"ok"}}}, nil
		}))
		response, err := executor.Execute(context.Background(), runner.Request{URL: "header://local"})
		if !b.closed || response.Body != nil {
			t.Fatalf("closed=%v response=%+v", b.closed, response)
		}
		if mode == "success" {
			if err != nil || len(response.Headers) != 1 {
				t.Fatalf("response=%+v err=%v", response, err)
			}
		} else {
			requireKind(t, err, Body)
			if len(response.Headers) != 0 || response.Status != 0 || strings.Contains(err.Error(), "private") {
				t.Fatalf("response=%+v err=%v", response, err)
			}
		}
	}
}

func TestHeaderWaitsForCompleteBody(t *testing.T) {
	for _, mode := range []string{"timeout", "cancel", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			received := make(chan struct{})
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				w.Header().Set("X-Test", "ok")
				w.Header().Set("Content-Length", "100")
				_, _ = io.WriteString(w, "short")
				if mode == "incomplete" {
					return
				}
				w.(http.Flusher).Flush()
				close(received)
				<-r.Context().Done()
			}))
			defer server.Close()
			s, err := parser.Parse("header.hurl", strings.NewReader("GET "+server.URL+"\nHTTP *\n[Asserts]\nheader \"X-Test\" == \"ok\"\nGET "+server.URL+"/next"))
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
					t.Fatal("headers not received")
				}
			}
			select {
			case got := <-done:
				kind := Timeout
				if mode == "cancel" {
					kind = Canceled
				}
				if mode == "incomplete" {
					kind = Body
				}
				requireKind(t, got.Err, kind)
				if hits.Load() != 1 || got.Summary.Completed != 0 || got.Entries[0].Outcome != runner.Failed || got.Diverged {
					t.Fatalf("got=%+v hits=%d", got, hits.Load())
				}
			case <-time.After(3 * time.Second):
				t.Fatal("runner did not finish")
			}
		})
	}
}

func TestWireHeadersPreserveOccurrences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		connection, buffer, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = connection.Close() }()
		_, _ = fmt.Fprint(buffer, "HTTP/1.1 200 OK\r\nX-Multi: first,whole\r\nx-mULTI: second\r\nX-Empty:\r\nContent-Length: 0\r\n\r\n")
		_ = buffer.Flush()
	}))
	defer server.Close()
	response, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	var multi []string
	for _, header := range response.Headers {
		if header.Name == "X-Multi" {
			multi = append(multi, header.Value)
		}
	}
	if !reflect.DeepEqual(multi, []string{"first,whole", "second"}) || response.Body != nil {
		t.Fatalf("response=%+v", response)
	}
	if value, present := query.Header(response.Headers, "x-empty"); !present || value != "" {
		t.Fatalf("empty=%q,%v", value, present)
	}
	if _, present := query.Header(response.Headers, "missing"); present {
		t.Fatal("invented header")
	}
}
