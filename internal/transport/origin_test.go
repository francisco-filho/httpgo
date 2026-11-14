package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestUppercaseSchemeRedirect(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/start" {
			w.Header().Set("Location", "HTTP://"+r.Host+"/end")
			w.WriteHeader(302)
			return
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	response, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{URL: strings.Replace(server.URL, "http:", "HTTP:", 1) + "/start"})
	if err != nil || response.Status != 204 || hits.Load() != 2 {
		t.Fatalf("response=%+v,err=%v,hits=%d", response, err, hits.Load())
	}
}
