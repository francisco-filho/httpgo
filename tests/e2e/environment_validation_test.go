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

func TestEnvironmentInvalidDocumentsNeverSend(t *testing.T) {
	t.Setenv("HTTPGO_ENV_TOKEN", "process-must-not-resolve")
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	dir := t.TempDir()
	base := configFile(t, dir, "base.yaml", "version: 1\nvariables: {a: 1}")
	file := configFile(t, t.TempDir(), "static.hurl", "GET "+server.URL+"\nHTTP 200\nGET "+server.URL+"/never\nHTTP 200")
	for _, source := range []string{
		"version: [", "version: 2", "version: '1'", "[]", "null", "variables: {}\n---", "version: 1\nversion: 1", "unknown: fake-private",
		"variables: []", "variables: {a: [!!str fake-private]}", "variables: {a: &a 1, b: *a}", "variables: {a: 1, a: 2}",
		"variables: {a: {child: 1}}", "variables: {bad-name: 1}", "variables: {a: .inf}", "variables: {a: 2026-11-15}",
		"secrets: [bad-name]", "defaults: {timeout: 0.0001}", "defaults: {timeout: 86400.001}", "defaults: {timeout: '2s'}",
		"variables: {a: '${HTTPGO_ENV_TOKEN}'}", "#" + strings.Repeat("x", 1048576),
	} {
		selected := environmentFile(t, dir, "local", source)
		code, out, stderr := runConfig(t, dir, "test", file, "--config", base, "--env", "local", "--var", "a=override")
		if code != 2 || out != "" || !strings.Contains(stderr, selected) || hits.Load() != 0 || strings.Contains(stderr, "fake-private") || strings.Contains(stderr, "process-must-not-resolve") {
			t.Fatalf("source prefix=%q code=%d out=%q err=%q hits=%d", source[:min(len(source), 60)], code, out, stderr, hits.Load())
		}
	}
	for _, flags := range [][]string{{"--env"}, {"--env="}, {"--env", "../local"}, {"--env", "a.b"}, {"--env", `a\b`}, {"--env", "a b"}, {"--env=local", "--env=local"}, {"--env=local", "--env=staging"}, {"--env=absent"}} {
		code, out, stderr := runConfig(t, dir, append([]string{"test", file, "--config", base}, flags...)...)
		if code != 2 || out != "" || stderr == "" || hits.Load() != 0 {
			t.Fatalf("flags=%v code=%d out=%q err=%q hits=%d", flags, code, out, stderr, hits.Load())
		}
	}
	unreadable := filepath.Join(dir, "environments", "unreadable.yaml")
	if err := os.Mkdir(unreadable, 0700); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := runConfig(t, dir, "test", file, "--config", base, "--env", "unreadable")
	if code != 2 || out != "" || stderr == "" || hits.Load() != 0 {
		t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
	}
	// With no selection, even oversized or malformed environment files are inert.
	code, out, stderr = runConfig(t, dir, "test", file, "--config", base)
	if code != 0 || stderr != "" || hits.Load() != 2 {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
}

func TestEnvironmentEmptyLimitAndNoAncestor(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	parent := t.TempDir()
	environmentFile(t, parent, "local", "invalid: [")
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	file := configFile(t, t.TempDir(), "static.hurl", "GET {{base}}\nHTTP 200")
	code, out, stderr := runConfig(t, child, "test", file, "--env", "local", "--var", "base="+server.URL)
	if code != 2 || out != "" || stderr == "" || hits.Load() != 0 {
		t.Fatalf("ancestor loaded: code=%d out=%q err=%q", code, out, stderr)
	}
	for _, source := range []string{"", " \t\n", "# comments", "#" + strings.Repeat("x", 1048575)} {
		environmentFile(t, child, "local", source)
		code, out, stderr := runConfig(t, child, "test", file, "--env", "local", "--var", "base="+server.URL)
		if code != 0 || stderr != "" {
			t.Fatalf("size=%d code=%d out=%q err=%q", len(source), code, out, stderr)
		}
	}
	if hits.Load() != 4 {
		t.Fatal(hits.Load())
	}
}

func TestEnvironmentPublicTimeout(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/start" {
			w.Header().Set("Location", "/blocked")
			w.WriteHeader(302)
			return
		}
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	dir := t.TempDir()
	base := configFile(t, dir, "base.yaml", "version: 1\ndefaults: {timeout: 3}")
	environmentFile(t, dir, "local", "defaults: {timeout: 0.125}")
	file := configFile(t, dir, "timeout.hurl", "GET "+server.URL+"/start\nHTTP 200\nGET "+server.URL+"/never\nHTTP 200")
	code, out, stderr := runConfig(t, dir, "test", file, "--config", base, "--env", "local")
	if code != 2 || hits.Load() != 2 || !strings.Contains(stderr, "limite total de 0.125 segundos") || !strings.Contains(out, "iniciadas=1 concluídas=0") || !strings.Contains(out, "não_iniciadas=1") {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
}
