package transport

import (
	"context"
	"encoding/json"
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

func TestTypedJSONOnWire(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		switch r.URL.Path {
		case "/products":
			if r.Header.Get("Content-Type") != "application/json" || string(body) != `{"name":"Pizza","price":4500,"active":true,"code":"001"}` {
				t.Errorf("headers=%v body=%s", r.Header, body)
			}
		case "/root":
			var value []json.RawMessage
			if err := json.Unmarshal(body, &value); err != nil || len(value) != 3 || string(value[0]) != "null" || string(value[1]) != `{"big":9007199254740993123456789}` || string(value[2]) != `"{{literal}}"` {
				t.Errorf("root=%s err=%v", body, err)
			}
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(201)
	}))
	defer server.Close()
	source := "POST {{api.baseUrl}}/products\nContent-Type: application/json\n{\"name\":\"{{product.name}}\",\"price\":{{product.price}},\"active\":{{product.active}},\"code\":{{product.code}}}\nHTTP 201\nPOST {{api.baseUrl}}/root\n{{root}}\nHTTP 201"
	scenario, err := parser.Parse("products.hurl", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := template.New([]string{"api.baseUrl=" + server.URL, "product.name=Pizza", "product.price=4500", "product.active=true", `product.code="001"`, `root=[null,{"big":9007199254740993123456789},"{{literal}}"]`}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := runner.NewWithRenderer(NewNetHTTPExecutor(), renderer, nil).Run(context.Background(), scenario)
	if result.Err != nil || result.Diverged || hits.Load() != 2 || result.Summary.Validated != 2 {
		t.Fatalf("result=%+v hits=%d", result, hits.Load())
	}
}
