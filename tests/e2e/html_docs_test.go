package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHTMLDocumentedSuiteAndHelp(t *testing.T) {
	for _, path := range []string{"README.md", "docs/html.md", "docs/subset.md"} {
		data, err := os.ReadFile(filepath.Join("../..", path))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "--fail-fast --output html --output-file target/report.html") {
			t.Fatalf("%s omits HTML usage", path)
		}
	}
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
		for _, text := range []string{"--output html --output-file target/report.html", "YYYYMMDDTHHMMSSmmmZ", "UTC", "offline", "arquivo parcial", "segredos", "Sem HTML e JUnit simultâneos"} {
			if !strings.Contains(out, text) {
				t.Fatalf("help missing %q", text)
			}
		}
	}
	for _, fast := range []bool{false, true} {
		t.Run(map[bool]string{false: "continue", true: "fail-fast"}[fast], func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/products" {
					w.WriteHeader(503)
				}
			}))
			defer server.Close()
			base := filepath.Join(t.TempDir(), "report.html")
			args := []string{"test", "../../testdata/examples/html_suite/", "--var", "base_url=" + server.URL, "--output=html", "--output-file", base}
			if fast {
				args = append(args, "--fail-fast")
			}
			code, out, stderr := run(t, args...)
			_, html := htmlArtifact(t, base)
			want := int32(3)
			if fast {
				want = 2
			}
			if code != 1 || hits.Load() != want || out != "" || !strings.Contains(stderr, "gerado em") || !strings.Contains(html, "esperado 200, recebido 503") {
				t.Fatalf("code=%d hits=%d out=%q err=%q HTML=%s", code, hits.Load(), out, stderr, html)
			}
			if fast != strings.Contains(html, "<strong>não executado</strong>") {
				t.Fatal("wrong skipped file state")
			}
		})
	}
}
