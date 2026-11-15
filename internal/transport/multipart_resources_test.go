package transport

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

type replayUpload struct {
	*strings.Reader
	active      atomic.Bool
	concurrent  atomic.Bool
	reads       atomic.Int32
	seeks       int
	failure     error
	seekFailure error
}

func (f *replayUpload) Read(p []byte) (int, error) {
	if !f.active.CompareAndSwap(false, true) {
		f.concurrent.Store(true)
	}
	defer f.active.Store(false)
	f.reads.Add(1)
	if f.failure != nil {
		return 0, f.failure
	}
	return f.Reader.Read(p)
}
func (f *replayUpload) Seek(offset int64, whence int) (int64, error) {
	if f.active.Load() {
		f.concurrent.Store(true)
	}
	f.seeks++
	if f.seekFailure != nil && f.seeks > 1 {
		return 0, f.seekFailure
	}
	return f.Reader.Seek(offset, whence)
}

func uploadRequest(target string, source io.ReadSeeker, size int64) runner.Request {
	return runner.Request{Method: "POST", URL: target, Headers: []parser.Header{{Name: "Content-Type", Value: "multipart/form-data"}}, Multipart: &runner.MultipartBody{Parts: []runner.MultipartPart{{Name: "a", Value: "text"}, {Name: "document", File: true, Filename: "file.pdf", MIME: "application/pdf"}}, Source: source, Size: size}}
}

func TestMultipartRedirects(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			const payload = "\x00\xff{{missing}}"
			source := &replayUpload{Reader: strings.NewReader(payload)}
			var mu sync.Mutex
			var headers []string
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hit := hits.Add(1)
				if hit == 2 && status < 307 {
					data, err := io.ReadAll(r.Body)
					if r.Method != "GET" || len(data) != 0 || err != nil || r.Header.Get("Content-Type") != "" {
						t.Errorf("method=%s headers=%v body=%q err=%v", r.Method, r.Header, data, err)
					}
				} else {
					mu.Lock()
					headers = append(headers, r.Header.Get("Content-Type"))
					mu.Unlock()
					reader, err := r.MultipartReader()
					if err != nil {
						t.Error(err)
						return
					}
					first, err := reader.NextPart()
					if err != nil {
						t.Error(err)
						return
					}
					text, _ := io.ReadAll(first)
					file, err := reader.NextPart()
					if err != nil {
						t.Error(err)
						return
					}
					data, err := io.ReadAll(file)
					if err != nil || string(data) != payload || string(text) != "text" || r.Method != "POST" {
						t.Errorf("method=%s text=%q body=%q err=%v", r.Method, text, data, err)
					}
					_, _ = io.Copy(io.Discard, r.Body)
				}
				if hit == 1 {
					w.Header().Set("Location", "/final")
					w.WriteHeader(status)
				} else {
					w.WriteHeader(201)
				}
			}))
			defer server.Close()
			response, err := NewNetHTTPExecutor().Execute(context.Background(), uploadRequest(server.URL, source, int64(len(payload))))
			wantSeeks := 1
			if status >= 307 {
				wantSeeks = 2
			}
			if err != nil || response.Status != 201 || hits.Load() != 2 || source.seeks != wantSeeks || source.concurrent.Load() {
				t.Fatalf("response=%+v err=%v hits=%d seeks=%d concurrent=%t", response, err, hits.Load(), source.seeks, source.concurrent.Load())
			}
			mu.Lock()
			defer mu.Unlock()
			if status >= 307 && (len(headers) != 2 || headers[0] != headers[1]) {
				t.Fatalf("boundary changed=%v", headers)
			}
		})
	}
}

func TestMultipartEarlyResponseAndReadFailure(t *testing.T) {
	for _, mode := range []string{"early413", "read", "rewind", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch mode {
				case "early413":
					w.Header().Set("Content-Length", "0")
					w.WriteHeader(413)
					w.(http.Flusher).Flush()
					<-release
				case "disconnect":
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
				default:
					_, _ = io.Copy(io.Discard, r.Body)
					w.Header().Set("Location", "/again")
					w.WriteHeader(307)
				}
			}))
			defer server.Close()
			defer close(release)
			var source io.ReadSeeker
			size := int64(128 << 20)
			var controlled *replayUpload
			if mode == "early413" || mode == "disconnect" {
				source = &largeUpload{size: size}
			} else {
				controlled = &replayUpload{Reader: strings.NewReader("payload")}
				size = 7
				source = controlled
				if mode == "read" {
					controlled.failure = io.ErrUnexpectedEOF
				} else {
					controlled.seekFailure = io.ErrClosedPipe
				}
			}
			executor := NewNetHTTPExecutor()
			executor.timeout = 2 * time.Second
			response, err := executor.Execute(context.Background(), uploadRequest(server.URL, source, size))
			if mode == "early413" {
				if err != nil || response.Status != 413 {
					t.Fatalf("response=%+v err=%v cause=%v", response, err, errors.Unwrap(err))
				}
				return
			}
			if err == nil || hits.Load() > 1 {
				t.Fatalf("response=%+v err=%v hits=%d", response, err, hits.Load())
			}
			if mode == "read" && !errors.Is(err, io.ErrUnexpectedEOF) || mode == "rewind" && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("lost cause=%v", err)
			}
		})
	}
}

func TestMultipartNetworkPoliciesAndDeadline(t *testing.T) {
	for _, mode := range []string{"timeout", "cancel", "TLS", "incomplete", "cross", "limit"} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{}, 1)
			closed := make(chan struct{}, 16)
			var hits atomic.Int32
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				if mode == "cross" {
					w.Header().Set("Location", "http://localhost:"+strings.Split(r.Host, ":")[1])
					w.WriteHeader(307)
					return
				}
				if mode == "limit" {
					w.Header().Set("Location", "/again")
					w.WriteHeader(308)
					return
				}
				if mode == "incomplete" {
					w.Header().Set("Content-Length", "100")
					_, _ = io.WriteString(w, "short")
					return
				}
				if mode == "timeout" && r.URL.Path == "/start" {
					w.Header().Set("Location", "/wait")
					w.WriteHeader(307)
					return
				}
				started <- struct{}{}
				w.WriteHeader(200)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			server.Config.ErrorLog = log.New(io.Discard, "", 0)
			server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateClosed {
					closed <- struct{}{}
				}
			}
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
			done := make(chan error, 1)
			source := &replayUpload{Reader: strings.NewReader("payload")}
			go func() { _, err := executor.Execute(ctx, uploadRequest(server.URL+"/start", source, 7)); done <- err }()
			if mode == "cancel" {
				select {
				case <-started:
					cancel()
				case <-time.After(3 * time.Second):
					t.Fatal("not started")
				}
			}
			var err error
			select {
			case err = <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("upload did not terminate")
			}
			kind, wantHits := Timeout, int32(2)
			switch mode {
			case "cancel":
				kind, wantHits = Canceled, 1
			case "TLS":
				kind, wantHits = TLS, 0
			case "incomplete":
				kind, wantHits = Body, 1
			case "cross":
				kind, wantHits = Redirect, 1
			case "limit":
				kind, wantHits = Redirect, 11
			}
			requireKind(t, err, kind)
			if hits.Load() != wantHits || source.active.Load() || source.concurrent.Load() {
				t.Fatalf("hits=%d active=%t concurrent=%t", hits.Load(), source.active.Load(), source.concurrent.Load())
			}
			for _, deadline := range deadlines {
				if deadline.IsZero() || !deadline.Equal(deadlines[0]) {
					t.Fatalf("deadlines changed=%v", deadlines)
				}
			}
			if mode == "cancel" || mode == "timeout" {
				select {
				case <-closed:
				case <-time.After(3 * time.Second):
					t.Fatal("connection retained")
				}
			}
		})
	}
}
