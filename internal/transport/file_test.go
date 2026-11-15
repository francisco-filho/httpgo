package transport

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestExternalResponseConsumedAndClosed(t *testing.T) {
	for _, mode := range []string{"success", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			closed := make(chan struct{}, 1)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != "raw {{missing}}" {
					t.Errorf("body=%q err=%v", body, err)
				}
				if mode == "incomplete" {
					w.Header().Set("Content-Length", "1000")
				}
				_, _ = io.WriteString(w, "response")
			}))
			server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateClosed {
					closed <- struct{}{}
				}
			}
			server.Start()
			defer server.Close()
			path := filepath.Join(t.TempDir(), "payload")
			if err := os.WriteFile(path, []byte("raw {{missing}}"), 0600); err != nil {
				t.Fatal(err)
			}
			scenario, err := parser.Parse("source.hurl", strings.NewReader("POST "+server.URL+"\nfile,"+path+";\nHTTP 200"))
			if err != nil {
				t.Fatal(err)
			}
			result := runner.New(NewNetHTTPExecutor(), nil).Run(context.Background(), scenario)
			if mode == "incomplete" {
				requireKind(t, result.Err, Body)
			} else if result.Err != nil || result.Summary.Completed != 1 {
				t.Fatalf("result=%+v", result)
			}
			select {
			case <-closed:
			case <-time.After(3 * time.Second):
				t.Fatal("response connection not closed")
			}
		})
	}
}
