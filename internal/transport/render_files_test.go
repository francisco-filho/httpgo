package transport

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

func TestExternalRenderedRedirectsAndTLS(t *testing.T) {
	for _, tls := range []bool{false, true} {
		for _, status := range []int{301, 302, 303, 307, 308} {
			t.Run(fmt.Sprintf("tls=%t/%d", tls, status), func(t *testing.T) {
				var hits atomic.Int32
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					method, want := "POST", `{"id":9007199254740993,"name":"quotes\"\\\n雪"}`
					if r.URL.Path == "/final" && status <= 303 {
						method, want = "GET", ""
					}
					if err != nil || r.Method != method || string(body) != want {
						t.Errorf("method=%s body=%q want=%q err=%v", r.Method, body, want, err)
					}
					hits.Add(1)
					if r.URL.Path == "/start" {
						w.Header().Set("Location", "/final")
						w.WriteHeader(status)
						return
					}
					w.WriteHeader(201)
				}))
				if tls {
					server.StartTLS()
				} else {
					server.Start()
				}
				defer server.Close()
				path := filepath.Join(t.TempDir(), "payload.json")
				if err := os.WriteFile(path, []byte(`{"id":{{id}},"name":"{{name}}"}`), 0600); err != nil {
					t.Fatal(err)
				}
				scenario, err := parser.Parse("source.hurl", strings.NewReader("POST "+server.URL+"/start\nContent-Type: application/json\nfile,"+path+";\nHTTP 201"))
				if err != nil {
					t.Fatal(err)
				}
				renderer, err := template.New([]string{"id=9007199254740993", `name="quotes\"\\\n雪"`}, nil)
				if err != nil {
					t.Fatal(err)
				}
				executor := NewNetHTTPExecutor()
				if tls {
					executor.transport = server.Client().Transport.(*http.Transport)
				}
				got := runner.NewWithRenderer(executor, renderer, nil).RunWithOptions(context.Background(), scenario, runner.Options{RenderFiles: true})
				if got.Err != nil || got.Diverged || hits.Load() != 2 || got.Summary.Started != 1 || got.Summary.Validated != 1 {
					t.Fatalf("got=%+v hits=%d", got, hits.Load())
				}
			})
		}
	}
}
