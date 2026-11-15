package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExternalRenderedDocumentedScenarios(t *testing.T) {
	for _, tc := range []struct {
		scenario, payload string
		code, hits        int
		position, detail  string
	}{
		{"scenario_external_render.hurl", "product_template.json", 0, 1, "", ""},
		{"scenario_external_render_unsupported_key.hurl", "external_render_dynamic_key.json", 2, 1, ":2:4:", "chave JSON"},
		{"scenario_external_render_malformed.hurl", "external_render_malformed.json", 2, 1, ":2:12:", "fechamento ausente"},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("..", "..", "testdata", "scenarios", tc.scenario))
			if err != nil {
				t.Fatal(err)
			}
			payload, err := os.ReadFile(filepath.Join("..", "..", "testdata", "payloads", tc.payload))
			if err != nil {
				t.Fatal(err)
			}
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.Method == "POST" {
					body, err := io.ReadAll(r.Body)
					if err != nil || string(body) != "{\"name\":\"Pizza\",\"price\":45,\"active\":true}\n" {
						t.Errorf("body=%q err=%v", body, err)
					}
					w.WriteHeader(201)
					return
				}
				w.WriteHeader(200)
			}))
			defer server.Close()
			dir := t.TempDir()
			for _, name := range []string{"scenarios", "payloads"} {
				if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "payloads", tc.payload), payload, 0600); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, "scenarios", tc.scenario)
			if err := os.WriteFile(file, []byte(strings.ReplaceAll(string(input), "http://localhost:8080", server.URL)), 0600); err != nil {
				t.Fatal(err)
			}
			code, out, stderr := run(t, "test", file, "--render-files", "--var", "product.name=Pizza", "--var", "product.price=45", "--var", "product.active=true", "--var", "key=static")
			if code != tc.code || int(hits.Load()) != tc.hits || tc.code == 0 && stderr != "" || tc.code != 0 && (!strings.Contains(stderr, fmt.Sprintf("%q%s", "../payloads/"+tc.payload, tc.position)) || !strings.Contains(stderr, tc.detail)) {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		for _, text := range []string{"[--render-files]", "extensão HTTPGo", "Não renderiza arquivos multipart", "somente na vez da requisição", "payload/linha/coluna originais", "Não promete renderização equivalente pelo Hurl"} {
			if code != 0 || stderr != "" || !strings.Contains(out, text) {
				t.Fatalf("help missing %q code=%d out=%q err=%q", text, code, out, stderr)
			}
		}
	}
}
