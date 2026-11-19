package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCapturesGuideCount(t *testing.T) {
	input := guideBlock(t, "docs/captures.md", "Salve `contagem.httpgo`", "hurl")
	for _, tc := range []struct {
		name, first, last string
		code, calls       int
		render            bool
	}{
		{"unchanged", `{"items":[1,null,3]}`, `{"items":[4,5,6]}`, 0, 3, true},
		{"changed", `{"items":[1,2,3]}`, `{"items":[1,2,3,4]}`, 1, 3, true},
		{"empty array", `{"items":[]}`, `{"items":[]}`, 0, 3, true},
		{"missing capture", `{}`, ``, 2, 1, true},
		{"null capture", `{"items":null}`, ``, 2, 1, true},
		{"scalar capture", `{"items":3}`, ``, 2, 1, true},
		{"missing assertion", `{"items":[]}`, `{}`, 1, 3, true},
		{"without opt in", ``, ``, 2, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				if n == 2 {
					body, err := io.ReadAll(r.Body)
					if err != nil || r.Method != "POST" || r.URL.Path != "/orders/preview" || string(body) != `{"productId":"p1"}` {
						t.Errorf("request=%s %s body=%q err=%v", r.Method, r.URL.Path, body, err)
					}
					return
				}
				if r.Method != "GET" || r.URL.Path != "/orders" {
					t.Errorf("request=%s %s", r.Method, r.URL.Path)
				}
				body := tc.first
				if n == 3 {
					body = tc.last
				}
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			file := writeReadmeFile(t, t.TempDir(), "contagem.httpgo", input)
			args := []string{"test", file, "--var", "base_url=" + server.URL}
			if tc.render {
				args = append(args, "--render-asserts")
			}
			code, out, stderr := run(t, args...)
			if code != tc.code || int(calls.Load()) != tc.calls || (code == 0 && stderr != "") || (code != 0 && stderr == "") {
				t.Fatalf("code=%d want=%d calls=%d want=%d out=%q stderr=%q", code, tc.code, calls.Load(), tc.calls, out, stderr)
			}
			if tc.calls == 1 && !strings.Contains(stderr, "erro de captura") {
				t.Errorf("capture error missing: %s", stderr)
			}
		})
	}
}
