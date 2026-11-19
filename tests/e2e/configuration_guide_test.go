package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestConfigurationGuidePrecedence(t *testing.T) {
	var hits [3]atomic.Int32
	var addresses [3]string
	for i := range addresses {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits[i].Add(1)
			if r.Method != "GET" || r.URL.Path != "/health" {
				t.Errorf("request=%s %s", r.Method, r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"status":"ok"}`)
		}))
		t.Cleanup(server.Close)
		addresses[i] = server.URL
	}
	dir := t.TempDir()
	base := strings.ReplaceAll(guideBlock(t, "docs/configuration.md", "salve `.httpgo.yaml`:", "yaml"), "http://localhost:8080", addresses[0])
	local := strings.ReplaceAll(guideBlock(t, "docs/configuration.md", "Crie `environments/local.yaml`:", "yaml"), "http://localhost:8081", addresses[1])
	writeReadmeFile(t, dir, ".httpgo.yaml", base)
	writeReadmeFile(t, dir, "environments/local.yaml", local)
	writeReadmeFile(t, dir, "exemplo.httpgo", guideBlock(t, "docs/configuration.md", "Salve `exemplo.httpgo`:", "hurl"))
	for i, extra := range [][]string{nil, {"--env", "local"}, {"--env", "local", "--var", "base_url=" + addresses[2]}} {
		code, out, stderr := runConfig(t, dir, append([]string{"test", "exemplo.httpgo"}, extra...)...)
		if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") {
			t.Fatalf("variant=%d code=%d out=%q stderr=%q", i, code, out, stderr)
		}
		for j := range hits {
			want := int32(0)
			if j <= i {
				want = 1
			}
			if hits[j].Load() != want {
				t.Fatalf("variant=%d server=%d hits=%d want=%d", i, j, hits[j].Load(), want)
			}
		}
	}
	for _, extra := range [][]string{
		{"--var", `product={"name":"Outro"}`},
		{"--var", "base_url=" + addresses[0], "--var", "base_url=" + addresses[0]},
	} {
		code, out, stderr := runConfig(t, dir, append([]string{"test", "exemplo.httpgo"}, extra...)...)
		if code != 2 || stderr == "" || out != "" {
			t.Fatalf("conflict args=%v code=%d out=%q stderr=%q", extra, code, out, stderr)
		}
	}
	for i := range hits {
		if hits[i].Load() != 1 {
			t.Fatalf("conflict sent HTTP to server %d: hits=%d", i, hits[i].Load())
		}
	}
}
