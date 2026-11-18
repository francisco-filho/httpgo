package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// Read the published input rather than maintaining a second copy in the test.
func readmeBlock(t *testing.T, marker, language string) string {
	t.Helper()
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, tail, ok := strings.Cut(string(data), marker)
	if !ok {
		t.Fatalf("README marker missing: %q", marker)
	}
	_, tail, ok = strings.Cut(tail, "```"+language+"\n")
	if !ok {
		t.Fatalf("README block missing after %q", marker)
	}
	body, _, ok := strings.Cut(tail, "\n```")
	if !ok {
		t.Fatalf("README block not closed after %q", marker)
	}
	return body + "\n"
}

func TestReadmeExternalRendering(t *testing.T) {
	template := readmeBlock(t, "salve `payloads/product-template.json`", "text")
	input := readmeBlock(t, "Salve `tests/renderizado.hurl`:", "hurl")
	for _, render := range []bool{false, true} {
		t.Run(map[bool]string{false: "literal bytes", true: "rendered JSON"}[render], func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				data, err := io.ReadAll(r.Body)
				want := strings.TrimSuffix(template, "\n")
				if render {
					want = `{"name":"Pizza","price":4500}`
				}
				if err != nil || strings.TrimSuffix(string(data), "\n") != want || r.Method != "POST" || r.URL.Path != "/products" || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("request=%s %s body=%q want=%q err=%v", r.Method, r.URL.Path, data, want, err)
				}
				if render {
					w.WriteHeader(201)
				} else {
					w.WriteHeader(400)
				}
			}))
			defer server.Close()
			dir := t.TempDir()
			for _, name := range []string{"tests", "payloads"} {
				if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "payloads", "product-template.json"), []byte(template), 0600); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, "tests", "renderizado.hurl")
			if err := os.WriteFile(file, []byte(strings.ReplaceAll(input, "http://localhost:8080", server.URL)), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"test", file, "--var", "product.name=Pizza", "--var", "product.price=4500"}
			wantCode := 1
			if render {
				args = append(args, "--render-files")
				wantCode = 0
			}
			code, out, stderr := run(t, args...)
			if code != wantCode || calls.Load() != 1 || (render && stderr != "") || (!render && !strings.Contains(stderr, "status divergente")) {
				t.Fatalf("code=%d want=%d calls=%d out=%q stderr=%q", code, wantCode, calls.Load(), out, stderr)
			}
		})
	}
}

func TestReadmeAssertionsAndCaptures(t *testing.T) {
	for _, tc := range []struct {
		name, marker, from, to, response string
		code, calls                      int
		dynamic                          bool
	}{
		{name: "headers JSON and text", marker: "Salve `validacoes.hurl`:", calls: 1},
		{name: "JSON divergence", marker: "Salve `validacoes.hurl`:", from: `== "Pizza"`, to: `== "Suco"`, code: 1, calls: 1},
		{name: "invalid JSON", marker: "Salve `validacoes.hurl`:", response: `not JSON`, code: 2, calls: 1},
		{name: "capture and reuse", marker: "Salve `encadeamento.hurl`.", calls: 2},
		{name: "dynamic comparison", marker: "Salve `encadeamento.hurl`.", calls: 2, dynamic: true},
		{name: "discard on divergence", marker: "Salve `encadeamento.hurl`.", from: `jsonpath "$.id" == 1`, to: `jsonpath "$.id" == 2`, code: 1, calls: 1},
		{name: "capture absent", marker: "Salve `encadeamento.hurl`.", response: `{}`, code: 2, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				body := `{"id":1,"name":"Pizza","price":4500}`
				switch {
				case r.Method == "POST" && r.URL.Path == "/products":
					data, err := io.ReadAll(r.Body)
					if err != nil || string(data) != `{"name":"Pizza","price":4500}` {
						t.Errorf("POST body=%q err=%v", data, err)
					}
					w.WriteHeader(201)
					body = `{"id":1}`
				case r.Method == "GET" && r.URL.Path == "/products/1":
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if tc.response != "" {
					body = tc.response
				}
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			input := readmeBlock(t, tc.marker, "hurl")
			if tc.from != "" {
				if !strings.Contains(input, tc.from) {
					t.Fatalf("expected assertion missing: %s", tc.from)
				}
				input = strings.Replace(input, tc.from, tc.to, 1)
			}
			if tc.dynamic {
				input += "jsonpath \"$.id\" == {{product_id}}\n"
			}
			args := []string{"test", scenario(t, input), "--var", "base_url=" + server.URL}
			if tc.dynamic {
				args = append(args, "--render-asserts")
			}
			code, out, stderr := run(t, args...)
			if code != tc.code || int(calls.Load()) != tc.calls || (code == 0 && stderr != "") || (code != 0 && stderr == "") {
				t.Fatalf("code=%d want=%d calls=%d want=%d out=%q stderr=%q", code, tc.code, calls.Load(), tc.calls, out, stderr)
			}
		})
	}
}
