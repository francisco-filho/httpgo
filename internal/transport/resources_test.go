package transport

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestConnectionsCloseOnSuccessAndBodyFailure(t *testing.T) {
	for _, mode := range []string{"success", "incomplete", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			closed := make(chan struct{}, 1)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "incomplete":
					w.Header().Set("Content-Length", "100")
					_, _ = io.WriteString(w, "short")
				case "timeout":
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				default:
					_, _ = io.WriteString(w, strings.Repeat("payload", 10000))
				}
			}))
			server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateClosed {
					closed <- struct{}{}
				}
			}
			server.Start()
			defer server.Close()
			executor := NewNetHTTPExecutor()
			executor.timeout = 150 * time.Millisecond
			response, err := executor.Execute(context.Background(), runner.Request{URL: server.URL})
			switch mode {
			case "incomplete":
				requireKind(t, err, Body)
			case "timeout":
				requireKind(t, err, Timeout)
			default:
				if err != nil || response.Status != 200 {
					t.Fatalf("response=%+v,err=%v", response, err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			select {
			case <-closed:
			case <-ctx.Done():
				t.Fatal("connection was not closed after Execute")
			}
		})
	}
}
