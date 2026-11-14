package transport

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

type receivedRequest struct {
	Method, Body string
	Headers      http.Header
	Host         string
}

func receive(t *testing.T, r *http.Request) receivedRequest {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
	}
	return receivedRequest{r.Method, string(body), r.Header.Clone(), r.Host}
}

func TestMethodsHeadersAndExactJSON(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE", "GET"} {
		bodies := []string{`{"name":"Pizza","price":4500}`, "{\r\n  \"nested\": {}\r\n}", "[\n 1, true, null\n]", `"HTTP 200 # [Asserts]"`, `42`, `false`, `null`, ""}
		if method == "GET" {
			bodies = []string{""}
		}
		for _, body := range bodies {
			t.Run(method+body, func(t *testing.T) {
				got := make(chan receivedRequest, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got <- receive(t, r); w.WriteHeader(201) }))
				defer server.Close()
				response, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{Method: method, URL: server.URL, Body: []byte(body), Headers: []parser.Header{{Name: "Content-Type", Value: "application/json"}, {Name: "Authorization", Value: "Bearer header-secret"}, {Name: "X-Repeat", Value: "one"}, {Name: "x-repeat", Value: "two"}, {Name: "Host", Value: "virtual.example"}}})
				if err != nil || response.Status != 201 {
					t.Fatalf("response=%+v,error=%v", response, err)
				}
				received := <-got
				if received.Method != method || received.Body != body || received.Headers.Get("Content-Type") != "application/json" || received.Headers.Get("Authorization") != "Bearer header-secret" || !reflect.DeepEqual(received.Headers.Values("X-Repeat"), []string{"one", "two"}) || received.Host != "virtual.example" {
					t.Fatalf("unexpected request: %+v", received)
				}
			})
		}
	}
}

func TestRedirectMethodsAndBodies(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
			t.Run(fmt.Sprintf("%d/%s", status, method), func(t *testing.T) {
				body := "{\n \"password\": \"body-secret\"\n}"
				if method == "GET" {
					body = ""
				}
				var mu sync.Mutex
				var requests []receivedRequest
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					current := receive(t, r)
					mu.Lock()
					requests = append(requests, current)
					mu.Unlock()
					if r.URL.Path == "/start" {
						w.Header().Set("Location", "/middle")
						w.WriteHeader(status)
						return
					}
					if r.URL.Path == "/middle" {
						w.Header().Set("Location", "/final")
						w.WriteHeader(307)
						return
					}
					w.WriteHeader(201)
				}))
				defer server.Close()
				got, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{Method: method, URL: server.URL + "/start", Body: []byte(body), Headers: []parser.Header{{Name: "Authorization", Value: "Bearer header-secret"}}})
				if err != nil || got.Status != 201 {
					t.Fatalf("response=%+v,error=%v", got, err)
				}
				mu.Lock()
				defer mu.Unlock()
				if len(requests) != 3 {
					t.Fatalf("requests=%+v", requests)
				}
				wantMethod, wantBody := method, body
				if status <= 303 && method != "GET" {
					wantMethod, wantBody = "GET", ""
				}
				if requests[0].Method != method || requests[0].Body != body {
					t.Fatalf("initial request=%+v", requests[0])
				}
				for _, req := range requests[1:] {
					if req.Method != wantMethod || req.Body != wantBody || req.Headers.Get("Authorization") != "Bearer header-secret" {
						t.Fatalf("redirected request=%+v,want method=%s,body=%q", req, wantMethod, wantBody)
					}
				}
			})
		}
	}
}

func TestPreservedBodyRedirectBoundaries(t *testing.T) {
	for _, mode := range []string{"allowed ten", "blocked eleven", "cross origin", "no retry"} {
		t.Run(mode, func(t *testing.T) {
			var hits, targetHits, next atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { targetHits.Add(1); w.WriteHeader(200) }))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/next" {
					next.Add(1)
					return
				}
				hits.Add(1)
				received := receive(t, r)
				if received.Method != "PATCH" || received.Body != `{"a":1}` {
					t.Errorf("request=%+v", received)
				}
				if mode == "no retry" {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
					return
				}
				n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
				if mode == "allowed ten" && n == 10 {
					w.WriteHeader(201)
					return
				}
				destination := fmt.Sprintf("/%d", n+1)
				if mode == "cross origin" {
					destination = target.URL
				}
				w.Header().Set("Location", destination)
				w.WriteHeader(308)
			}))
			defer server.Close()
			input := "PATCH " + server.URL + "/0\n{\"a\":1}\nHTTP 201\nDELETE " + server.URL + "/next"
			scenario, err := parser.Parse("redirect.hurl", strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			result := runner.New(NewNetHTTPExecutor(), nil).Run(context.Background(), scenario)
			wantHits := int32(11)
			wantNext := int32(0)
			switch mode {
			case "allowed ten":
				wantNext = 1
				if result.Err != nil || result.Diverged {
					t.Fatalf("result=%+v", result)
				}
			case "blocked eleven":
				requireKind(t, result.Err, Redirect)
			case "cross origin":
				wantHits = 1
				requireKind(t, result.Err, Redirect)
			case "no retry":
				wantHits = 1
				requireKind(t, result.Err, Network)
			}
			if hits.Load() != wantHits || targetHits.Load() != 0 || next.Load() != wantNext {
				t.Fatalf("hits=%d,target=%d,next=%d", hits.Load(), targetHits.Load(), next.Load())
			}
		})
	}
}
