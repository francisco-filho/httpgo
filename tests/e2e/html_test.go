package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func htmlArtifact(t *testing.T, base string) (string, string) {
	t.Helper()
	ext := filepath.Ext(base)
	paths, err := filepath.Glob(strings.TrimSuffix(base, ext) + "-*" + ext)
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	return paths[0], string(data)
}

func TestHTMLRealDemonstration(t *testing.T) {
	for _, mode := range []string{"pass", "assertion", "scenario error", "single"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/products" && mode == "assertion" {
					w.WriteHeader(503)
				}
			}))
			defer server.Close()
			dir := t.TempDir()
			first := configFile(t, dir, "01-health.hurl", "GET "+server.URL+"/health\nHTTP 200")
			middle := "GET " + server.URL + "/products\nHTTP 200"
			if mode == "scenario error" {
				middle += "\nHEAD " + server.URL
			}
			configFile(t, dir, "02-products.hurl", middle)
			configFile(t, dir, "03-profile.hurl", "GET "+server.URL+"/profile\nHTTP 200")
			input, wantCode, wantHits := dir, 0, int32(3)
			switch mode {
			case "single":
				input, wantHits = first, 1
			case "assertion":
				wantCode, wantHits = 1, 2
			case "scenario error":
				wantCode, wantHits = 2, 1
			}
			base := filepath.Join(t.TempDir(), "target", "report.html")
			code, out, stderr := run(t, "test", input, "--fail-fast", "--output", "html", "--output-file", base)
			path, html := htmlArtifact(t, base)
			if code != wantCode || hits.Load() != wantHits || out != "" || !strings.Contains(stderr, fmt.Sprintf("relatório HTML gerado em %q", path)) || !strings.Contains(html, server.URL+"/health") {
				t.Fatalf("code=%d hits=%d out=%q stderr=%q HTML=%s", code, hits.Load(), out, stderr, html)
			}
			if mode == "assertion" && (!strings.Contains(html, "esperado 200, recebido 503") || !strings.Contains(html, "<strong>não executado</strong>")) {
				t.Fatal(html)
			}
		})
	}
}

func TestHTMLMissingDestinationPreventsRealHTTP(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL), "--output=html")
	if code != 2 || hits.Load() != 0 || out != "" || !strings.Contains(stderr, "HTML exige") {
		t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
	}
}
