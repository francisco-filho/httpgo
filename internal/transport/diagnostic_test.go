package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestDiagnosticSafeTransportCategory(t *testing.T) {
	cause := errors.New("raw-private-cause")
	err := &Error{Kind: TLS, cause: cause}
	var safe interface{ SafeDiagnostic() string }
	if !errors.As(err, &safe) || safe.SafeDiagnostic() != string(TLS) || !errors.Is(err, cause) {
		t.Fatalf("safe category or original cause lost: %v", err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	scenario, parseErr := parser.Parse("tls.hurl", strings.NewReader("GET "+server.URL+"\nHTTP 200"))
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	result := runner.New(NewNetHTTPExecutor(), nil).RunWithOptions(context.Background(), scenario, runner.Options{DiagnosticBodies: true})
	if result.Err == nil || len(result.Entries) != 1 || result.Entries[0].Diagnostic == nil || result.Entries[0].Diagnostic.Detail != string(TLS) {
		t.Fatalf("safe category not delivered: %+v", result)
	}
}

func TestDiagnosticBodiesAreFinalRedirectHop(t *testing.T) {
	for _, code := range []int{301, 302, 303, 307, 308} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var mu sync.Mutex
			var methods, bodies []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				mu.Lock()
				methods = append(methods, r.Method)
				bodies = append(bodies, string(data))
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(code)
					_, _ = io.WriteString(w, `{"intermediate":"never-display"}`)
					return
				}
				w.WriteHeader(503)
				_, _ = io.WriteString(w, `{"final":true}`)
			}))
			defer server.Close()
			response, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{Method: "POST", URL: server.URL + "/start", Body: []byte(`{"sent":true}`), DiagnosticBodies: true})
			if err != nil || response.Diagnostics == nil || response.Status != 503 || string(response.Body) != `{"final":true}` {
				t.Fatalf("response=%+v err=%v", response, err)
			}
			diag := response.Diagnostics
			mu.Lock()
			defer mu.Unlock()
			if len(methods) != 2 || string(diag.Response.Bytes) != `{"final":true}` {
				t.Fatalf("methods=%v diag=%+v", methods, diag)
			}
			if code < 307 {
				if methods[1] != "GET" || bodies[1] != "" || diag.Request.State != runner.BodyAbsent || len(diag.Request.Bytes) != 0 {
					t.Fatalf("methods=%v bodies=%v diag=%+v", methods, bodies, diag)
				}
			} else if methods[1] != "POST" || string(diag.Request.Bytes) != bodies[1] || bodies[1] != `{"sent":true}` {
				t.Fatalf("methods=%v bodies=%v diag=%+v", methods, bodies, diag)
			}
		})
	}
}

func TestDiagnosticStatusOnlyConsumesCompleteResponse(t *testing.T) {
	for _, mode := range []string{"complete", "incomplete", "timeout", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if mode == "incomplete" {
					w.Header().Set("Content-Length", "100")
				}
				w.WriteHeader(200)
				_, _ = io.WriteString(w, `{"private-partial":true}`)
				if mode == "timeout" || mode == "cancel" {
					w.(http.Flusher).Flush()
					close(started)
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan struct{})
			if mode == "cancel" {
				go func() {
					defer close(finished)
					select {
					case <-started:
						cancel()
					case <-ctx.Done():
					}
				}()
			}
			req := runner.Request{URL: server.URL, Timeout: 80 * time.Millisecond, DiagnosticBodies: true}
			response, err := NewNetHTTPExecutor().Execute(ctx, req)
			if mode == "cancel" {
				cancel()
				<-finished
			}
			if mode == "complete" {
				if err != nil || response.Diagnostics == nil || string(response.Body) != `{"private-partial":true}` {
					t.Fatalf("response=%+v err=%v", response, err)
				}
			} else if err == nil || response.Diagnostics == nil || response.Diagnostics.Response.State != runner.BodyUnavailable || len(response.Diagnostics.Response.Bytes) != 0 || len(response.Body) != 0 {
				t.Fatalf("partial response exposed: %+v err=%v", response, err)
			}
		})
	}
}

func TestDiagnosticReadAndCloseBeforeReturning(t *testing.T) {
	for _, mode := range []string{"complete", "read", "close"} {
		t.Run(mode, func(t *testing.T) {
			reader := strings.NewReader(`{"private":true}`)
			body := &captureBody{Reader: reader}
			if mode == "read" {
				body.Reader = io.MultiReader(reader, errorReader{})
			}
			if mode == "close" {
				body.closeErr = errors.New("private-close-cause")
			}
			executor := NewNetHTTPExecutor()
			executor.transport.RegisterProtocol("capture", captureRoundTripper(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: body}, nil
			}))
			response, err := executor.Execute(context.Background(), runner.Request{URL: "capture://local", DiagnosticBodies: true})
			if !body.closed || reader.Len() != 0 || response.Diagnostics == nil {
				t.Fatalf("closed=%t unread=%d response=%+v", body.closed, reader.Len(), response)
			}
			if mode == "complete" {
				if err != nil || response.Status != 200 || string(response.Diagnostics.Response.Bytes) != `{"private":true}` {
					t.Fatalf("response=%+v err=%v", response, err)
				}
			} else if err == nil || response.Status != 0 || response.Diagnostics.Response.State != runner.BodyUnavailable || len(response.Diagnostics.Response.Bytes) != 0 {
				t.Fatalf("incomplete exposed: response=%+v err=%v", response, err)
			}
		})
	}
}

func TestDiagnosticRetentionOptInAndRunnerSafety(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		_, _ = io.WriteString(w, `{"response":"safe"}`)
	}))
	defer server.Close()
	for _, enabled := range []bool{false, true} {
		response, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{URL: server.URL, DiagnosticBodies: enabled})
		if err != nil || enabled != (response.Diagnostics != nil) || enabled != (len(response.Body) > 0) {
			t.Fatalf("enabled=%t response=%+v err=%v", enabled, response, err)
		}
		scenario, err := parser.Parse("diagnostic.hurl", strings.NewReader("POST "+server.URL+"\n{\"sent\":true}\nHTTP 200"))
		if err != nil {
			t.Fatal(err)
		}
		reports := 0
		result := runner.New(NewNetHTTPExecutor(), func(entry runner.EntryResult) error {
			reports++
			if entry.Entry.Body != nil || enabled != (entry.Bodies != nil) {
				t.Fatalf("entry=%+v", entry)
			}
			if enabled && (entry.Bodies.Request.Text != `{"sent":true}` || entry.Bodies.Response.Text != `{"response":"safe"}`) {
				t.Fatalf("preview=%+v", entry.Bodies)
			}
			return nil
		}).RunWithOptions(context.Background(), scenario, runner.Options{DiagnosticBodies: enabled})
		if reports != 1 || !result.Diverged || result.Err != nil {
			t.Fatalf("result=%+v", result)
		}
	}
}
