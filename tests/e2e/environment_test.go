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

func environmentFile(t *testing.T, dir, name, text string) string {
	t.Helper()
	root := filepath.Join(dir, "environments")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	return configFile(t, root, name+".yaml", text)
}

func TestEnvironmentDemonstration(t *testing.T) {
	var hits atomic.Int32
	server := func(want string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			body, err := io.ReadAll(r.Body)
			if err != nil || r.Method != "POST" || r.URL.Path != "/products" || string(body) != want {
				t.Errorf("method=%s path=%s body=%s want=%s err=%v", r.Method, r.URL.Path, body, want, err)
			}
			w.WriteHeader(201)
		}))
	}
	local := server(`{"name":"Pizza","price":4500,"active":true}`)
	defer local.Close()
	staging := server(`{"name":"Suco","price":1200,"active":true}`)
	defer staging.Close()
	root := t.TempDir()
	base := configFile(t, root, ".httpgo.yaml", "version: 1\nvariables: {product: {active: true}}")
	environmentFile(t, root, "local", fmt.Sprintf("variables: {api: {baseUrl: %s}, product: {name: Pizza, price: 4500}}", local.URL))
	environmentFile(t, root, "staging", fmt.Sprintf("version: 1\nvariables: {api: {baseUrl: %s}, product: {name: Suco, price: 1200}}", staging.URL))
	file := configFile(t, t.TempDir(), "products.hurl", "POST {{api.baseUrl}}/products\nContent-Type: application/json\n{\"name\":\"{{product.name}}\",\"price\":{{product.price}},\"active\":{{product.active}}}\nHTTP 201")
	for _, name := range []string{"local", "staging"} {
		code, out, stderr := runConfig(t, t.TempDir(), "test", file, "--config", base, "--env", name)
		if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") {
			t.Fatalf("env=%s code=%d out=%q err=%q", name, code, out, stderr)
		}
	}
	if hits.Load() != 2 {
		t.Fatal(hits.Load())
	}
}

func TestEnvironmentRoots(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	root := t.TempDir()
	base := configFile(t, root, "empty.yaml", "")
	environmentFile(t, root, "local", "variables: {base: "+server.URL+"}")
	scenarioDir := t.TempDir()
	environmentFile(t, scenarioDir, "local", "invalid: [")
	file := configFile(t, scenarioDir, "request.hurl", "GET {{base}}\nHTTP 200")
	cwd := t.TempDir()
	environmentFile(t, cwd, "local", "variables: {base: "+server.URL+"}")
	for _, flags := range [][]string{{"--config", base, "--env", "local"}, {"--env=local"}} {
		code, out, stderr := runConfig(t, cwd, append([]string{"test", file}, flags...)...)
		if code != 0 || stderr != "" {
			t.Fatalf("flags=%v code=%d out=%q err=%q", flags, code, out, stderr)
		}
	}
	// No fallback from a missing selected environment to base or scenario.
	code, out, stderr := runConfig(t, cwd, "test", file, "--config", base, "--env", "missing")
	if code != 2 || out != "" || stderr == "" || hits.Load() != 2 {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
}
