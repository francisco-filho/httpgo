package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func configFile(t *testing.T, dir, name, text string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func runConfig(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary(t), args...)
	command.Dir = dir
	var out, stderr bytes.Buffer
	command.Stdout = &out
	command.Stderr = &stderr
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("harness deadline: %v", ctx.Err())
	}
	code := 0
	if err != nil {
		if command.ProcessState == nil {
			t.Fatal(err)
		}
		code = command.ProcessState.ExitCode()
	}
	return code, out.String(), stderr.String()
}

func TestConfigDiscoveryAndSelection(t *testing.T) {
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
	dir := t.TempDir()
	base := configFile(t, dir, ".httpgo.yaml", fmt.Sprintf("version: 1\nvariables:\n  api: {baseUrl: %s}\n  product: {price: 4500}\n", server.URL))
	file := configFile(t, dir, "products.hurl", "POST {{api.baseUrl}}/products\nContent-Type: application/json\n{\"price\": {{product.price}}}\nHTTP 201")
	code, out, stderr := runConfig(t, dir, "test", file)
	if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") || hits.Load() != 1 {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
	external := configFile(t, t.TempDir(), "selected.yaml", fmt.Sprintf("version: 1\nvariables: {api.baseUrl: %s, product.price: 4500}", server.URL))
	if err := os.WriteFile(base, []byte("malformed: ["), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr = runConfig(t, dir, "test", "--config", external, file)
	if code != 0 || stderr != "" || hits.Load() != 2 {
		t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
	}
	code, out, stderr = runConfig(t, dir, "test", file, "--config", filepath.Join(dir, "missing.yaml"))
	if code != 2 || out != "" || stderr == "" || hits.Load() != 2 {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
}

func TestConfigEmptyAbsentAndNoAncestor(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	parent := t.TempDir()
	configFile(t, parent, ".httpgo.yaml", "invalid: [")
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	file := configFile(t, child, "static.hurl", "GET {{base}}\nHTTP 200")
	for _, source := range []string{"absent", "", " \n# comment only\n"} {
		if source != "absent" {
			configFile(t, child, ".httpgo.yaml", source)
		}
		code, out, stderr := runConfig(t, child, "test", file, "--var", "base="+server.URL)
		if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") {
			t.Fatalf("source=%q code=%d out=%q err=%q", source, code, out, stderr)
		}
	}
	if hits.Load() != 3 {
		t.Fatal(hits.Load())
	}
}
