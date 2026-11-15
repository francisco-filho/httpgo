package transport

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestWireHeadersPreserveOccurrences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		connection, buffer, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = connection.Close() }()
		_, _ = fmt.Fprint(buffer, "HTTP/1.1 200 OK\r\nX-Multi: first,whole\r\nx-mULTI: second\r\nX-Empty:\r\nContent-Length: 0\r\n\r\n")
		_ = buffer.Flush()
	}))
	defer server.Close()
	response, err := NewNetHTTPExecutor().Execute(context.Background(), runner.Request{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	var multi []string
	for _, header := range response.Headers {
		if header.Name == "X-Multi" {
			multi = append(multi, header.Value)
		}
	}
	if !reflect.DeepEqual(multi, []string{"first,whole", "second"}) || response.Body != nil {
		t.Fatalf("response=%+v", response)
	}
	if value, present := query.Header(response.Headers, "x-empty"); !present || value != "" {
		t.Fatalf("empty=%q,%v", value, present)
	}
	if _, present := query.Header(response.Headers, "missing"); present {
		t.Fatal("invented header")
	}
}
