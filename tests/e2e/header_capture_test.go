package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHeaderCaptureCacheByBinary(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/scenario_capture_cache.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ETag", "etag", "eTaG"} {
		t.Run(name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.Header.Get("If-None-Match") == `"catalog-v1"` {
					w.WriteHeader(304)
					return
				}
				w.Header().Set("ETag", `"catalog-v1"`)
				_, _ = io.WriteString(w, "not JSON")
			}))
			defer server.Close()
			source := strings.ReplaceAll(string(input), `"ETag"`, `"`+name+`"`)
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base_url="+server.URL)
			if code != 0 || stderr != "" || hits.Load() != 2 || !strings.Contains(out, "validadas=2") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
}

func TestHeaderCaptureLiteralAndNoCookieStoreByBinary(t *testing.T) {
	for _, value := range []string{"", "123", "true", "null", `{"x":1}`, "{{missing}}", "first,whole"} {
		t.Run(value, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := hits.Add(1)
				if n == 1 {
					w.Header().Add("X-Value", value)
					w.Header().Add("X-Value", "ignored")
					w.Header().Set("Set-Cookie", "session=private-cookie; Path=/; HttpOnly")
					_, _ = io.WriteString(w, "not JSON")
					return
				}
				if r.Header.Get("Cookie") != "" {
					t.Error("implicit cookie store")
				}
				body, _ := io.ReadAll(r.Body)
				var got map[string]string
				if r.Header.Get("X-Value") != value || json.Unmarshal(body, &got) != nil || got["full"] != value || got["text"] != "pre"+value || got["cookie"] != "session=private-cookie; Path=/; HttpOnly" {
					t.Error("literal string changed")
				}
			}))
			defer server.Close()
			source := "GET {{base}}/first\n[Captures]\nx: header \"X-Value\"\ncookie: header \"Set-Cookie\"\nPOST {{base}}/next\nX-Value: {{x}}\n{\"full\":{{x}},\"text\":\"pre{{x}}\",\"cookie\":{{cookie}}}"
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL)
			if code != 0 || stderr != "" || hits.Load() != 2 || strings.Contains(out+stderr, "private-cookie") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
}
