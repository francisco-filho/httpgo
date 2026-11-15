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

func TestConfigDocumentedFixturesAndOverrides(t *testing.T) {
	t.Setenv("HTTPGO_TEST_TOKEN", "fake-process-value")
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil || r.URL.Path != "/products" || string(body) != `{"price": 4500}` {
			t.Errorf("path=%s body=%s err=%v", r.URL.Path, body, err)
		}
		w.WriteHeader(201)
	}))
	defer server.Close()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := filepath.Join(root, "testdata", "scenarios", "scenario_config_products.hurl")
	configFile(t, dir, ".httpgo.yaml", "malformed: [")
	code, out, stderr := runConfig(t, dir, "test", file, "--config", filepath.Join(root, "testdata", "config", "project.yaml"), "--var", "api.baseUrl="+server.URL)
	if code != 0 || stderr != "" || hits.Load() != 1 || !strings.Contains(out, "validadas=1") {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
	for _, name := range []string{"invalid_duplicate.yaml", "unsupported_environment.yaml", "unsupported_anchor.yaml", "unsupported_tag.yaml"} {
		code, out, stderr := runConfig(t, dir, "test", file, "--config", filepath.Join(root, "testdata", "config", name), "--var", "api.baseUrl="+server.URL, "--var", "token=fake-override")
		if code != 2 || out != "" || stderr == "" || hits.Load() != 1 {
			t.Fatalf("name=%s code=%d out=%q err=%q hits=%d", name, code, out, stderr, hits.Load())
		}
	}
}

func TestConfigInvalidDocumentsNeverSend(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	dir := t.TempDir()
	file := configFile(t, dir, "static.hurl", "GET "+server.URL+"\nHTTP 200\nGET "+server.URL+"/never\nHTTP 200")
	for i, source := range []string{
		"version: [", "variables: {}", "version: 99", "[]", "null", "version: 1\n---\nversion: 1", "version: 1\nversion: 1", "version: 1\nunknown: fake-private", "version: 1\nvariables: []", "version: 1\nvariables: {a: [1, !!str fake-private]}", "version: 1\nvariables: {a: 1, a.child: 2}", "version: 1\nvariables: {a: .inf}", "version: 1\nvariables: {a: 2026-11-15}", "version: 1\nsecrets: [bad-name]", "version: 1\ndefaults: {timeout: 0.0001}", "version: 1\ndefaults: {timeout: 86400.001}", "version: 1\ndefaults: {timeout: '2s'}", "version: 1\nvariables: {a: '${HOME}'}", "#" + strings.Repeat("x", 1048576),
	} {
		selected := configFile(t, dir, fmt.Sprintf("invalid-%d.yaml", i), source)
		code, out, stderr := runConfig(t, dir, "test", file, "--config", selected)
		if code != 2 || out != "" || !strings.Contains(stderr, selected) || hits.Load() != 0 || strings.Contains(stderr, "fake-private") {
			t.Fatalf("case=%d code=%d out=%q err=%q hits=%d", i, code, out, stderr, hits.Load())
		}
	}
	// Exactly one MiB remains valid, and metadata-only size checks are not used.
	source := "version: 1\n#"
	selected := configFile(t, dir, "limit.yaml", source+strings.Repeat("x", 1048576-len(source)))
	code, out, stderr := runConfig(t, dir, "test", file, "--config", selected)
	if code != 0 || stderr != "" || hits.Load() != 2 {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
	// Nonexistent/unreadable selected files must not fall back to discovery.
	if err := os.Mkdir(filepath.Join(dir, "directory.yaml"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, selected := range []string{filepath.Join(dir, "absent.yaml"), filepath.Join(dir, "directory.yaml")} {
		code, out, stderr := runConfig(t, dir, "test", file, "--config", selected)
		if code != 2 || out != "" || stderr == "" || hits.Load() != 2 {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}
