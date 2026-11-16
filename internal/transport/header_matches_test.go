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

// GO-283/284: final headers are usable only after complete response consumption.
func TestHeaderMatchesCompleteFinalResponse(t *testing.T) {
	for _, mode := range []string{"final", "intermediate only", "mismatch", "incomplete", "timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			received, closed := make(chan struct{}), make(chan struct{})
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/start":
					w.Header().Set("X-Test", "intermediate")
					w.Header().Set("Location", "/final")
					w.WriteHeader(302)
				case "/final":
					if mode != "intermediate only" {
						w.Header().Set("X-Test", "final")
					}
					w.Header().Set("ETag", "captured")
					if mode == "incomplete" || mode == "timeout" || mode == "cancel" {
						w.Header().Set("Content-Length", "100")
					}
					_, _ = io.WriteString(w, "not JSON")
					if mode == "timeout" || mode == "cancel" {
						w.(http.Flusher).Flush()
						close(received)
						<-r.Context().Done()
						close(closed)
					}
				case "/next":
					if r.Header.Get("If-None-Match") != "captured" {
						t.Error("capture not published")
					}
				default:
					t.Errorf("path=%q", r.URL.Path)
				}
			}))
			defer server.Close()
			pattern := "^final$"
			if mode == "intermediate only" {
				pattern = "^intermediate$"
			}
			if mode == "mismatch" {
				pattern = "^intermediate$"
			}
			s, err := parser.Parse("matches.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP *\n[Asserts]\nheader \"X-Test\" matches \""+pattern+"\"\n[Captures]\nx: header \"ETag\"\nGET "+server.URL+"/next\nIf-None-Match: {{x}}"))
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
			go func() {
				done <- runner.New(executor, func(e runner.EntryResult) error {
					if len(e.Entry.Assertions) > 0 && e.Entry.Assertions[0].Header.Compiled != nil {
						t.Error("compiled regexp in callback")
					}
					return nil
				}).Run(ctx, s)
			}()
			if mode == "cancel" {
				select {
				case <-received:
					cancel()
				case <-ctx.Done():
					t.Fatal("no headers")
				}
			}
			var got runner.Result
			select {
			case got = <-done:
			case <-time.After(4 * time.Second):
				t.Fatal("runner did not finish")
			}
			switch mode {
			case "final":
				if got.Err != nil || got.Diverged || hits.Load() != 3 || got.Summary.Completed != 2 {
					t.Fatalf("result=%+v hits=%d", got, hits.Load())
				}
			case "intermediate only", "mismatch":
				if got.Err != nil || !got.Diverged || hits.Load() != 2 || got.Summary.Completed != 1 {
					t.Fatalf("result=%+v hits=%d", got, hits.Load())
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
				if got.Diverged || got.Summary.Completed != 0 || hits.Load() != 2 || got.Entries[0].HeaderFailure != nil {
					t.Fatalf("result=%+v hits=%d", got, hits.Load())
				}
				if mode == "timeout" || mode == "cancel" {
					select {
					case <-closed:
					case <-time.After(time.Second):
						t.Fatal("connection not closed")
					}
				}
			}
		})
	}
}

func TestHeaderMatchesReadCloseAndDiscard(t *testing.T) {
	for _, mode := range []string{"success", "read", "close"} {
		reader := strings.NewReader(strings.Repeat("not JSON", 1000))
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
			return &http.Response{StatusCode: 200, Header: http.Header{"Etag": {`"exact"`}}, Body: body}, nil
		}))
		s, err := parser.Parse("matches.hurl", strings.NewReader("GET http://example.test\nHTTP 200\n[Asserts]\nheader \"ETag\" matches \"exact\"\nGET http://example.test/never"))
		if err != nil {
			t.Fatal(err)
		}
		reports := 0
		got := runner.New(headerCaptureExecutor{executor: executor, t: t}, func(e runner.EntryResult) error {
			reports++
			if !body.closed || reader.Len() != 0 {
				t.Error("report before body consumption and close")
			}
			if e.Entry.Assertions[0].Header.Compiled != nil {
				t.Error("operational regexp retained")
			}
			return errors.New("controlled report failure")
		}).Run(context.Background(), s)
		if reports != 1 || calls != 1 || got.ReportErr == nil || got.Summary.NotStarted != 1 {
			t.Fatalf("mode=%s result=%+v calls=%d reports=%d", mode, got, calls, reports)
		}
		if mode == "success" {
			if got.Err != nil || got.Summary.Completed != 1 || got.Entries[0].Outcome != runner.Passed {
				t.Fatalf("result=%+v", got)
			}
		} else {
			requireKind(t, got.Err, Body)
			if got.Summary.Completed != 0 || got.Diverged || got.Entries[0].HeaderFailure != nil {
				t.Fatalf("result=%+v", got)
			}
		}
	}
}
