package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestJUnitDocumentedExample(t *testing.T) {
	source, err := os.ReadFile("../../testdata/scenarios/scenario_junit.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"README.md", "docs/junit.md", "docs/subset.md"} {
		data, err := os.ReadFile(filepath.Join("../..", path))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "--output junit --output-file target/test-results.xml") {
			t.Fatalf("%s omits JUnit usage", path)
		}
		if path == "README.md" && !strings.Contains(string(data), "```hurl\n"+string(source)+"```") {
			t.Fatal("README example differs from executable fixture")
		}
	}
	for _, status := range []int{200, 503} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					t.Errorf("path=%q want /health", r.URL.Path)
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "target", "test-results.xml")
			code, out, stderr := run(t, "test", "../../testdata/scenarios/scenario_junit.hurl", "--var", "base_url="+server.URL, "--output", "junit", "--output-file", path)
			doc, raw := junitXML(t, path)
			want := 0
			if status != 200 {
				want = 1
			}
			if code != want || out != "" || doc.Tests != 1 || doc.Failures != want || (want == 0 && stderr != "") || (want == 1 && !strings.Contains(stderr, "status divergente")) {
				t.Fatalf("code=%d out=%q stderr=%q XML=%s", code, out, stderr, raw)
			}
		})
	}
}
