package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-430,431,438,440: real binary and HTTP, including typed reuse without opt-in.
func TestCountCaptureDemonstration(t *testing.T) {
	for _, tc := range []struct {
		name, before, after, tail string
		enabled                   bool
		code, hits                int
	}{
		{"preserved", `["private-order",null,null]`, `[1,2,3]`, "", true, 0, 4},
		{"changed", `[1,2,3]`, `[1,2,3,4]`, "", true, 1, 3},
		{"empty", `[]`, `[]`, "", true, 0, 4},
		{"not array", `null`, `[]`, "", true, 2, 1},
		{"literal rejects", `[]`, `[]`, "", false, 2, 0},
		{"late invalid", `[]`, `[]`, "\n[Captures]\nbad: jsonpath \"$\" count count", true, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := hits.Add(1)
				if r.URL.Path == "/orders" {
					body := tc.before
					if n > 1 {
						body = tc.after
					}
					_, _ = io.WriteString(w, `{"items":`+body+`}`)
				}
			}))
			defer server.Close()
			source := "GET {{base}}/orders\nHTTP 200\n[Captures]\norders_before: jsonpath \"$.items\" count\nPOST {{base}}/orders/preview\nContent-Type: application/json\n{\"productId\":\"p1\"}\nHTTP 200\nGET {{base}}/orders\nHTTP 200\n[Asserts]\njsonpath \"$.items\" count == {{orders_before}}\nGET {{base}}/end" + tc.tail
			args := []string{"test", scenario(t, source), "--var", "base=" + server.URL}
			if tc.enabled {
				args = append(args, "--render-asserts")
			}
			code, out, stderr := run(t, args...)
			if code != tc.code || int(hits.Load()) != tc.hits || strings.Contains(out+stderr, "private-order") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}

func TestCountCaptureNumericBodyWithoutRenderAsserts(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method == "GET" {
			_, _ = io.WriteString(w, `[null,null]`)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != `{"n":2,"text":"2"}` || r.Header.Get("X-Count") != "2" || r.URL.Path != "/2" {
			t.Errorf("body=%s err=%v headers=%v path=%s", body, err, r.Header, r.URL.Path)
		}
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", scenario(t, "GET {{base}}/\nHTTP 200\n[Captures]\nn: jsonpath \"$\" count\nPOST {{base}}/{{n}}\nX-Count: {{n}}\n{\"n\":{{n}},\"text\":\"{{n}}\"}\nHTTP 200"), "--var", "base="+server.URL)
	if code != 0 || hits.Load() != 2 {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}
