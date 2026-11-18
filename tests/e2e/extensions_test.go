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

func TestScenarioExtensionsDemo(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	dir := t.TempDir()
	text := "GET " + server.URL + "/health\nHTTP 200"
	a := configFile(t, dir, "01-health.httpgo", text)
	b := configFile(t, dir, "02-health.hurl", text)
	for _, tc := range []struct {
		paths []string
		files int
	}{{[]string{a}, 1}, {[]string{b}, 1}, {[]string{dir}, 2}, {[]string{b, dir, a, dir}, 2}} {
		hits.Store(0)
		code, out, stderr := run(t, append([]string{"test"}, tc.paths...)...)
		if code != 0 || stderr != "" || int(hits.Load()) != tc.files || !strings.Contains(out, fmt.Sprintf("resumo suíte: arquivos=%d processados=%d executados=%d sucessos=%d", tc.files, tc.files, tc.files, tc.files)) {
			t.Fatalf("paths=%v code=%d out=%q stderr=%q hits=%d", tc.paths, code, out, stderr, hits.Load())
		}
		if tc.files == 2 && strings.Index(out, "01-health.httpgo") >= strings.Index(out, "02-health.hurl") {
			t.Fatalf("not globally ordered: %q", out)
		}
	}
}

func TestScenarioExtensionEquivalentValidation(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("X-Result", "actual")
		w.WriteHeader(200)
	}))
	defer server.Close()
	for _, tc := range []struct {
		name, suffix, diagnostic string
		code, sends              int
	}{
		{"success", "", "", 0, 1},
		{"assertion", "\n[Asserts]\nheader \"X-Result\" == \"expected\"", "expected", 1, 1},
		{"invalid", "\nGET " + server.URL + "\nHTTP 999", "inválid", 2, 0},
		{"unsupported", "\nHEAD " + server.URL, "não suportad", 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, extension := range []string{".httpgo", ".hurl"} {
				hits.Store(0)
				file := configFile(t, t.TempDir(), "scenario"+extension, "GET "+server.URL+"\nHTTP 200"+tc.suffix)
				code, out, stderr := run(t, "test", file)
				if code != tc.code || int(hits.Load()) != tc.sends || !strings.Contains(stderr, tc.diagnostic) || (tc.code == 0 && stderr != "") || (tc.code == 2 && !prevalidationSummary(out)) {
					t.Fatalf("extension=%s code=%d out=%q stderr=%q hits=%d", extension, code, out, stderr, hits.Load())
				}
			}
		})
	}
}

func TestScenarioOnlyHTTPGoRecursiveAndExplicitExtensions(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	dir := t.TempDir()
	text := "GET " + server.URL + "\nHTTP 200"
	configFile(t, dir, "01.httpgo", text)
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	configFile(t, nested, "02.httpgo", text)
	explicit := []string{configFile(t, dir, "helper.txt", text), configFile(t, dir, "no-extension", text)}
	configFile(t, dir, "payload.json", "not a scenario")
	for _, tc := range []struct {
		paths []string
		sends int
	}{{[]string{dir}, 2}, {explicit, 2}, {append([]string{dir}, explicit...), 4}} {
		hits.Store(0)
		code, out, stderr := run(t, append([]string{"test"}, tc.paths...)...)
		if code != 0 || stderr != "" || int(hits.Load()) != tc.sends || !strings.Contains(out, fmt.Sprintf("sucessos=%d", tc.sends)) {
			t.Fatalf("paths=%v code=%d out=%q stderr=%q hits=%d", tc.paths, code, out, stderr, hits.Load())
		}
	}
}
