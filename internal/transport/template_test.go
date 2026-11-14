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
	"capimgrosso.com/httpgo/internal/template"
)

func TestRenderedRequestOnWire(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || r.RequestURI != "/a%2Fb?q=a/b" || r.Header.Get("X-Probe") != "{{literal}}=true" || r.Host != "example.test:8080" || string(body) != `{ "value": false }` {
			t.Errorf("request=%+v body=%q", r, body)
		}
		w.WriteHeader(201)
	}))
	defer server.Close()
	scenario, err := parser.Parse("wire.hurl", strings.NewReader("POST {{base}}/{{path}}?q=a/b\nX-Probe: {{value}}\nHost: {{host}}\n{ \"value\": false }\nHTTP 201\nGET {{missing}}\nGET {{base}}/last"))
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := template.New([]string{"base=" + server.URL, "path=a%2Fb", "value={{literal}}=true", "host=example.test:8080"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := runner.NewWithRenderer(NewNetHTTPExecutor(), renderer, nil).Run(context.Background(), scenario)
	if hits.Load() != 1 || got.Err == nil || got.Summary.Validated != 1 || got.Summary.Started != 1 {
		t.Fatalf("hits=%d result=%+v", hits.Load(), got)
	}
}
