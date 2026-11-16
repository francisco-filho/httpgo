package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPollingByBinary(t *testing.T) {
	for _, method := range []string{"GET", "POST"} {
		for _, ready := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/%v", method, ready), func(t *testing.T) {
				var jobs, next atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/next" {
						next.Add(1)
						w.WriteHeader(204)
						return
					}
					n := jobs.Add(1)
					body, err := io.ReadAll(r.Body)
					if err != nil || r.Method != method || r.Header.Get("X-Job") != "123" || method == "POST" && string(body) != `{"id":"123"}` {
						t.Errorf("request method=%s body=%q err=%v", r.Method, body, err)
					}
					if ready && n >= 3 {
						_, _ = io.WriteString(w, `{"status":"READY","value":"next"}`)
					} else {
						_, _ = io.WriteString(w, `{"status":"PENDING"}`)
					}
				}))
				defer server.Close()
				payload := ""
				if method == "POST" {
					payload = "{\"id\":\"123\"}\n"
				}
				input := method + " " + server.URL + "/job\nX-Job: 123\n[Options]\nretry: 3\nretry-interval: 0\n" + payload + "HTTP 200\n[Asserts]\njsonpath \"$.status\" == \"READY\"\n[Captures]\nvalue: jsonpath \"$.value\"\nGET " + server.URL + "/{{value}}\nHTTP 204\n"
				code, out, stderr := run(t, "test", scenario(t, input))
				wantCode, wantJobs, wantNext := 1, int32(4), int32(0)
				if ready {
					wantCode, wantJobs, wantNext = 0, 3, 1
				}
				if code != wantCode || jobs.Load() != wantJobs || next.Load() != wantNext || !strings.Contains(out, fmt.Sprintf("[tentativas=%d]", wantJobs)) || !strings.Contains(out, "tentativas_iniciadas=4") {
					t.Fatalf("code=%d jobs=%d next=%d out=%s err=%s", code, jobs.Load(), next.Load(), out, stderr)
				}
				if ready && stderr != "" || !ready && (!strings.Contains(stderr, "PENDING") || strings.Contains(stderr, "erro de captura")) {
					t.Fatalf("out=%s err=%s", out, stderr)
				}
			})
		}
	}
}

func TestPollingInvalidLateOptionsByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, option := range []string{"retry: -1", "retry: 1.5", "retry: 1\nretry: 2", "retry-interval: 9223372036855", "timeout: 5"} {
		code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/first\nHTTP 200\nGET "+server.URL+"/job\n[Options]\n"+option+"\nHTTP 200\n"))
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, fmt.Sprintf(":%d:1:", 5+strings.Count(option, "\n"))) {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
	}
}
