package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestHTMLRealRepeatedNamesPreserveArtifacts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	file := scenario(t, "GET "+server.URL+"\nHTTP 200")
	base := filepath.Join(t.TempDir(), "new", "report.with.dots.html")
	var previousPath string
	var previous []byte
	for range 2 {
		code, out, stderr := run(t, "test", file, "--output=html", "--output-file", base)
		paths, err := filepath.Glob(filepath.Join(filepath.Dir(base), "*.html"))
		if err != nil || code != 0 || out != "" || !strings.Contains(stderr, "relatório HTML gerado em") {
			t.Fatalf("code=%d out=%q err=%q paths=%v glob=%v", code, out, stderr, paths, err)
		}
		if previousPath != "" {
			data, err := os.ReadFile(previousPath)
			if err != nil || string(data) != string(previous) || len(paths) != 2 {
				t.Fatalf("old artifact changed: paths=%v err=%v", paths, err)
			}
		}
		for _, path := range paths {
			if !regexp.MustCompile(`^report\.with\.dots-\d{8}T\d{9}Z(?:-\d+)?\.html$`).MatchString(filepath.Base(path)) {
				t.Fatalf("invalid timestamp name %s", path)
			}
			if path != previousPath {
				if !strings.Contains(stderr, fmt.Sprintf("gerado em %q", path)) {
					t.Fatal(stderr)
				}
				previousPath = path
				previous, err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				break
			}
		}
	}
}
