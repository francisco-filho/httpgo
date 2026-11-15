package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestTypedCaptureUsesOnlyFinalResponse(t *testing.T) {
	for _, tls := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP", true: "TLS"}[tls], func(t *testing.T) {
			var hits atomic.Int32
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/create":
					w.Header().Set("Location", "/final")
					w.WriteHeader(307)
					_, _ = io.WriteString(w, `{"id":12,"data":"wrong"}`)
				case "/final":
					w.WriteHeader(201)
					_, _ = io.WriteString(w, `{"id":9007199254740993,"data":[null,true,1.2300e+99,{"literal":"{{missing}}"}]}`)
				case "/products/9007199254740993":
					body, _ := io.ReadAll(r.Body)
					if string(body) != `{"data":[null,true,1.2300e+99,{"literal":"{{missing}}"}],"id":9007199254740993}` || r.Header.Get("X-ID") != "9007199254740993" {
						t.Errorf("headers=%v body=%s", r.Header, body)
					}
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			})
			server := httptest.NewUnstartedServer(handler)
			if tls {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			executor := NewNetHTTPExecutor()
			if tls {
				executor.transport.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			}
			source := "POST " + server.URL + "/create\nHTTP 201\n[Captures]\nid: jsonpath \"$.id\"\nvalue: jsonpath \"$\"\nPOST " + server.URL + "/products/{{id}}\nX-ID: {{id}}\n{{value}}\nHTTP 200"
			scenario, err := parser.Parse("typed.hurl", strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			got := runner.New(executor, nil).Run(context.Background(), scenario)
			if got.Err != nil || got.Summary.Validated != 2 || hits.Load() != 3 {
				t.Fatalf("%+v hits=%d", got, hits.Load())
			}
		})
	}
}
