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

// GO-497: assertions consume only the complete final body, decoded by the
// existing charset policy. Transport failures cannot pass by negation.
func TestRegexAssertionFinalResponse(t *testing.T) {
	for _, mode := range []string{"utf8", "latin1", "intermediate only", "unknown charset", "invalid utf8", "incomplete", "timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			received := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
					_, _ = io.WriteString(w, "id=intermediate")
					return
				}
				if r.URL.Path == "/next" {
					return
				}
				body := []byte("id=café")
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				switch mode {
				case "latin1":
					body = []byte{'i', 'd', '=', 'c', 'a', 'f', 0xe9}
					w.Header().Set("Content-Type", "text/plain; charset=iso-8859-1")
				case "intermediate only":
					body = []byte("no value")
				case "unknown charset":
					w.Header().Set("Content-Type", "text/plain; charset=unknown")
				case "invalid utf8":
					body = []byte{0xff}
				case "incomplete", "timeout", "cancel":
					w.Header().Set("Content-Length", "100")
				}
				_, _ = w.Write(body)
				if mode == "timeout" || mode == "cancel" {
					w.(http.Flusher).Flush()
					close(received)
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			decl := `regex "id=(.*)" == "café"`
			if mode == "unknown charset" || mode == "invalid utf8" {
				decl = `regex "(x)" not contains "y"`
			}
			scenario, err := parser.Parse("regex.hurl", strings.NewReader("GET "+server.URL+"/start\nHTTP 200\n[Asserts]\n"+decl+"\nGET "+server.URL+"/next"))
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
			got := <-done
			wantHits := int32(2)
			switch mode {
			case "utf8", "latin1":
				wantHits = 3
				if got.Err != nil || got.Diverged {
					t.Fatalf("%+v", got)
				}
			case "intermediate only":
				if got.Err != nil || !got.Diverged || got.Entries[0].RegexFailure.Present {
					t.Fatalf("%+v", got)
				}
			case "unknown charset", "invalid utf8":
				if got.Err == nil || got.Entries[0].Outcome != runner.RegexFailed {
					t.Fatalf("%+v", got)
				}
			default:
				if got.Err == nil || got.Summary.Completed != 0 || got.Diverged {
					t.Fatalf("%+v", got)
				}
			}
			if hits.Load() != wantHits {
				t.Fatalf("hits=%d want=%d", hits.Load(), wantHits)
			}
		})
	}
}

func TestRegexAssertionReadAndClose(t *testing.T) {
	for _, mode := range []string{"success", "read", "close"} {
		t.Run(mode, func(t *testing.T) {
			reader := strings.NewReader("id=private-group")
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
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: body}, nil
			}))
			scenario, err := parser.Parse("regex.hurl", strings.NewReader("GET http://example.test\nHTTP 200\n[Asserts]\nregex \"id=(.*)\" exists\nGET http://example.test/next"))
			if err != nil {
				t.Fatal(err)
			}
			got := runner.New(regexCaptureExecutor{executor: executor, t: t, retain: true}, func(runner.EntryResult) error {
				if !body.closed || reader.Len() != 0 {
					t.Fatal("reported before consuming/closing")
				}
				return errors.New("stop report")
			}).Run(context.Background(), scenario)
			if calls != 1 || !body.closed || got.ReportErr == nil || (got.Err != nil) != (mode != "success") {
				t.Fatalf("calls=%d result=%+v", calls, got)
			}
		})
	}
}
