package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func prevalidationSummary(out string) bool {
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	return len(lines) == 2 && strings.HasPrefix(lines[0], "resumo ") && strings.Contains(lines[0], "total=desconhecido iniciadas=0 concluídas=0") && strings.Contains(lines[0], "resultado=erro") && strings.HasPrefix(lines[1], "resumo suíte: arquivos=1 processados=1 executados=0 sucessos=0 falhas_assertion=0 erros=1")
}

func suiteDisplayPath(t *testing.T, path string) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	name, err := filepath.Rel(cwd, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(name)
}

func TestSuiteDemoAndFailurePrecedence(t *testing.T) {
	for _, mode := range []string{"assertion", "parse", "transport", "mixed", "success"} {
		for _, fast := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", mode, fast), func(t *testing.T) {
				var mu sync.Mutex
				var paths []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					paths = append(paths, r.URL.Path)
					mu.Unlock()
					if r.URL.Path == "/products" && (mode == "assertion" || mode == "mixed") {
						w.WriteHeader(503)
						return
					}
					if r.URL.Path == "/products" && mode == "transport" {
						w.Header().Set("Content-Length", "100")
						w.WriteHeader(200)
						return
					}
					w.WriteHeader(200)
				}))
				defer server.Close()
				dir := t.TempDir()
				configFile(t, dir, "01-health.hurl", "GET "+server.URL+"/health\nHTTP 200")
				middle := "GET " + server.URL + "/products\nHTTP 200"
				if mode == "parse" {
					middle += "\nHEAD " + server.URL + "/unsupported"
				}
				configFile(t, dir, "02-products.hurl", middle)
				last := "GET " + server.URL + "/profile\nHTTP 200"
				if mode == "mixed" {
					last += "\nHEAD " + server.URL + "/unsupported"
				}
				configFile(t, dir, "03-profile.hurl", last)
				args := []string{"test", dir}
				if fast {
					args = append(args, "--fail-fast")
				}
				code, out, stderr := run(t, args...)
				expectedCode := 0
				if mode != "success" {
					expectedCode = 2
				}
				if mode == "assertion" || mode == "mixed" && fast {
					expectedCode = 1
				}
				mu.Lock()
				got := append([]string(nil), paths...)
				mu.Unlock()
				expectedPaths := []string{"/health"}
				if mode != "parse" {
					expectedPaths = append(expectedPaths, "/products")
				}
				if (!fast || mode == "success") && mode != "mixed" {
					expectedPaths = append(expectedPaths, "/profile")
				}
				if code != expectedCode || !reflect.DeepEqual(got, expectedPaths) || !strings.Contains(out, "resumo suíte:") {
					t.Fatalf("code=%d out=%q err=%q paths=%v want=%v", code, out, stderr, got, expectedPaths)
				}
				if mode == "assertion" {
					want := "sucessos=2 falhas_assertion=1 erros=0 não_executados=0"
					if fast {
						want = "sucessos=1 falhas_assertion=1 erros=0 não_executados=1"
					}
					if !strings.Contains(out, want) {
						t.Fatalf("summary=%q want=%q", out, want)
					}
				}
			})
		}
	}
}

func TestSuiteDiscoveryRealHTTP(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()
	dir := t.TempDir()
	a := configFile(t, dir, "01.hurl", "GET "+server.URL+"/a\nHTTP 200")
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	b := configFile(t, nested, "02.hurl", "GET "+server.URL+"/b\nHTTP 200")
	explicit := configFile(t, dir, "explicit", "GET "+server.URL+"/c\nHTTP 200")
	configFile(t, dir, "ignore.json", "ignored")
	for _, args := range [][]string{{"test", dir, explicit, b, a}, {"test", b, explicit, a, dir}} {
		mu.Lock()
		paths = nil
		mu.Unlock()
		code, out, stderr := run(t, args...)
		mu.Lock()
		got := append([]string(nil), paths...)
		mu.Unlock()
		if code != 0 || stderr != "" || !reflect.DeepEqual(got, []string{"/a", "/c", "/b"}) || strings.Count(out, "resumo ") != 4 {
			t.Fatalf("code=%d out=%q err=%q paths=%v", code, out, stderr, got)
		}
	}
	for _, bad := range []string{filepath.Join(dir, "absent"), t.TempDir()} {
		mu.Lock()
		paths = nil
		mu.Unlock()
		code, out, stderr := run(t, "test", dir, bad)
		mu.Lock()
		count := len(paths)
		mu.Unlock()
		if code != 2 || out != "" || stderr == "" || count != 0 {
			t.Fatalf("code=%d out=%q err=%q paths=%d", code, out, stderr, count)
		}
	}
}
